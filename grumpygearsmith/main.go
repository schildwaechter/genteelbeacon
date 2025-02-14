// Genteel Beacon - Grumpy Gearsmith
// Copyright Carsten Thiel 2025
//
// SPDX-Identifier: Apache-2.0

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var ErrNoNamespace = fmt.Errorf("Namespace not found")

// apparently this is how you get the pod's namespace...?
func GetNamespace() (string, error) {
	nsBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrNoNamespace
		}
		return "", err
	}
	ns := strings.TrimSpace(string(nsBytes))
	return ns, nil
}

func getBeacons(ns string, clientset *kubernetes.Clientset) ([]string, error) {
	deployments, err := clientset.AppsV1().Deployments(ns).List(context.TODO(), metav1.ListOptions{LabelSelector: "genteelbeacon"})
	if err != nil {
		slog.Error("Error getting deployments! " + err.Error())
		return nil, err
	}
	var deploymentNames []string

	for _, deploy := range deployments.Items {
		deploymentNames = append(deploymentNames, deploy.Name)
	}

	return deploymentNames, nil
}

func calcGearValues(beacon string, ns string, clientset *kubernetes.Clientset) (int64, float64, error) {
	pods, err := clientset.CoreV1().Pods(ns).List(context.TODO(), metav1.ListOptions{LabelSelector: "genteelbeacon=" + beacon})
	if err != nil {
		slog.Error("Eror getting pods for label genteelbeacon=" + beacon)
		return 0, 0, err
	}

	var number int64 = 0
	var sum float64 = 0
	for _, pod := range pods.Items {
		req, err := http.NewRequest("GET", "http://"+pod.Status.PodIP+":1333/metrics", nil)
		client := &http.Client{Timeout: 1 * time.Second}

		resp, err := client.Do(req)
		if err != nil {
			slog.Warn("Can't reach pod " + pod.Name)
			continue // we just ignore this pod
		}

		defer resp.Body.Close()
		var greaseVal float64
		responseScanner := bufio.NewScanner(resp.Body)
		var line string
		for responseScanner.Scan() {
			line = responseScanner.Text()
			if strings.HasPrefix(line, "genteelbeacon_greasefactor") {
				greaseReturn, err := strconv.ParseFloat(strings.TrimSpace(line[len("genteelbeacon_greasefactor"):]), 64)
				if err != nil {
					slog.Error("Error parsing prometheus value")
				}
				greaseVal = greaseReturn
			}
		}
		slog.Debug(fmt.Sprintf("GREASE for "+pod.Name+" is: %f\n", (greaseVal)))
		number += 1
		sum += greaseVal

	}

	if number == 0 {
		return 0, 0, errors.New("No relevant pods!")
	}
	return number, sum / float64(number), nil
}

func statsServe(w http.ResponseWriter, r *http.Request) {
	jsonString, _ := json.Marshal(gearStats)
	fmt.Fprint(w, string(jsonString))
}

func gearValueServe(w http.ResponseWriter, r *http.Request) {
	beacon := r.PathValue("beacon")
	if beacon == "" {
		slog.Warn("No beacon in query!")
	}
	beaconGear := gearStats[beacon]
	if beaconGear.Count == 0 {
		slog.Warn("Queried non-existent gear: " + beacon)
	}
	fmt.Fprint(w, beaconGear.Average)
}

type gearStat struct {
	Count   int64
	Average float64
}

var gearStats map[string]gearStat

func setGearStats() {
	for {
		config, err := rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			panic(err.Error())
		}
		ns, err := GetNamespace()
		if err != nil {
			panic(err.Error())
		}
		beacons, err := getBeacons(ns, clientset)
		if err != nil {
			slog.Error(err.Error())
		}
		for _, element := range beacons {
			number, average, err := calcGearValues(element, ns, clientset)
			if err != nil {
				slog.Error(err.Error())
			}
			gearStats[element] = gearStat{number, average}
			slog.Debug(fmt.Sprintf("Average for "+element+" is: %f\n", (average)))
		}
		time.Sleep(5 * time.Second)
	}
}

func main() {
	gearStats = make(map[string]gearStat)
	// run in background
	go setGearStats()

	router := http.NewServeMux()
	router.HandleFunc("/stats", statsServe)
	router.HandleFunc("/gearvalue/{beacon}", gearValueServe)
	server := http.Server{
		Addr:    ":8080",
		Handler: router,
	}
	server.ListenAndServe()

}
