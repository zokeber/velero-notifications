package controller

import (
	"os"
	"context"
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/zokeber/velero-notifications/notifications"
)

type VeleroController struct {
	Namespace        string
	Interval         time.Duration
	Verbose          bool
	Notifiers        []notifications.Notifier
	dynClient        dynamic.Interface
	processedBackups map[string]string
}

func NewVeleroController(namespace string, checkInterval int, verbose bool, notifiers []notifications.Notifier) (*VeleroController, error) {
	var kubeconfig *string
	var config *rest.Config
	var err error

	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(Optional) Absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "Absolute path to the kubeconfig file")
	}

	flag.Parse()

	if *kubeconfig != "" {
		if _, err := os.Stat(*kubeconfig); err == nil {
			config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
			if err != nil {
				log.Fatalf("Failed to build kubeconfig from flag: %v", err)
			}
			log.Println("Using local kubeconfig to connect to the cluster.")
		} else {
			config, err = rest.InClusterConfig()
			if err != nil {
				log.Fatalf("Failed to retrieve in-cluster kubeconfig: %v", err)
			}
			log.Println("Kubeconfig file not found. Using in-cluster configuration to connect to the Kubernetes API server.")
		}
	} else {
		config, err = rest.InClusterConfig()
		if err != nil {
			log.Fatalf("Failed to retrieve in-cluster kubeconfig: %v", err)
		}
		log.Println("Using in-cluster configuration to connect to the Kubernetes API server.")
	}

	dynClient, err := dynamic.NewForConfig(config)
	
	if err != nil {
		log.Fatalf("Error creating dynamic client: %v", err)
	}

	if verbose {
		log.Printf("Successfully connected to the Kubernetes API server in namespace '%s'.", namespace)
	}

	return &VeleroController{
		Namespace:        namespace,
		Interval:         time.Duration(checkInterval) * time.Second,
		Verbose:          verbose,
		Notifiers:        notifiers,
		dynClient:        dynClient,
		processedBackups: make(map[string]string),
	}, nil
}

func (vc *VeleroController) Run(ctx context.Context) {
	ticker := time.NewTicker(vc.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down Velero Controller.")
			return
		case <-ticker.C:
			vc.checkBackups()
		}
	}
}

func (vc *VeleroController) notifyAll(message string) {
	for _, notifier := range vc.Notifiers {
		if err := notifier.Notify(message); err != nil {
			log.Printf("Error sending notifications: %v", err)
		}
	}
}

func extractWarnings(obj map[string]interface{}) int {
	warnings := 0
	if w, found, err := unstructured.NestedFieldCopy(obj, "status", "warnings"); err == nil && found {
		switch v := w.(type) {
		case int:
			warnings = v
		case int64:
			warnings = int(v)
		case float64:
			warnings = int(v)
		case string:
			if val, err := strconv.Atoi(v); err == nil {
				warnings = val
			}
		}
	}
	return warnings
}

func (vc *VeleroController) checkBackups() {
	backupsGVR := schema.GroupVersionResource{
		Group:    "velero.io",
		Version:  "v1",
		Resource: "backups",
	}

	list, err := vc.dynClient.Resource(backupsGVR).Namespace(vc.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Printf("Error retrieving backups from Velero: %v", err)
		vc.notifyAll(fmt.Sprintf("Error retrieving backups from Velero: %v", err))
		return
	}

	if vc.Verbose {
		log.Printf("Found %d backups in namespace '%s'.", len(list.Items), vc.Namespace)
	}

	for _, item := range list.Items {
		backupName, _, _ := unstructured.NestedString(item.Object, "metadata", "name")
		phase, found, err := unstructured.NestedString(item.Object, "status", "phase")
		if err != nil || !found {
			log.Printf("Backup %s is not supported.", backupName)
			continue
		}

		if _, exists := vc.processedBackups[backupName]; !exists {
			if phase == "InProgress" {
				vc.processedBackups[backupName] = phase
				if vc.Verbose {
					log.Printf("New backup detected in progress: %s.", backupName)
				}
			}
			continue
		}

		prevState := vc.processedBackups[backupName]
		
		if prevState == "InProgress" && phase != "InProgress" {
			completionTimestamp, found, err := unstructured.NestedString(item.Object, "status", "completionTimestamp")
			if err != nil || !found {
				completionTimestamp = "Unknown"
			}
			startTimestamp, found, err := unstructured.NestedString(item.Object, "status", "startTimestamp")
			if err != nil || !found {
				startTimestamp = "Unknown"
			}
			progress, found, err := unstructured.NestedMap(item.Object, "status", "progress")
			itemsBackedUp := "Unknown"
			totalItems := "Unknown"
			if found && err == nil {
				if ib, ok := progress["itemsBackedUp"]; ok {
					itemsBackedUp = fmt.Sprintf("%v", ib)
				}
				if ti, ok := progress["totalItems"]; ok {
					totalItems = fmt.Sprintf("%v", ti)
				}
			}

			warnings := extractWarnings(item.Object)
			failureReason := ""

			if phase == "Failed" {
				if fr, found, err := unstructured.NestedString(item.Object, "status", "failureReason"); err == nil && found {
					failureReason = fr
				}
			}

			var message string
			if phase == "Completed" {
				message = fmt.Sprintf("Backup %s completed successfully.\nStart Time: %s, End Time: %s.\nProgress: %s/%s items processed", backupName, startTimestamp, completionTimestamp, itemsBackedUp, totalItems)
			} else {
				message = fmt.Sprintf("Backup %s finished with status: %s.\nStart Time: %s, End Time: %s.\nProgress: %s/%s items processed", backupName, phase, startTimestamp, completionTimestamp, itemsBackedUp, totalItems)
				if failureReason != "" {
					message += fmt.Sprintf("\nFailure Reason: %s", failureReason)
				}
			}
			
			if warnings > 0 {
				message += fmt.Sprintf(" (with %d warnings).", warnings)
			}
			
			log.Printf(message)
			vc.notifyAll(message)

			vc.processedBackups[backupName] = phase
		}

		if vc.Verbose && phase == "InProgress" {
			log.Printf("Backup %s is still in progress.", backupName)
		}
	}
}