// Genteel Beacon - Grumpy Gearsmith
// Copyright Carsten Thiel 2025
//
// SPDX-Identifier: Apache-2.0

package gearsmith

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
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
var nameSpace string

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

func getBeacons(clientset *kubernetes.Clientset) ([]string, error) {
	deployments, err := clientset.AppsV1().Deployments(nameSpace).List(context.TODO(), metav1.ListOptions{LabelSelector: "genteelbeacon"})
	if err != nil {
		slog.Error("Error getting deployments! " + err.Error())
		return nil, err
	}
	var deploymentNames []string

	for _, deploy := range deployments.Items {
		deploymentNames = append(deploymentNames, deploy.Name)
	}

	slog.Debug(fmt.Sprintf("Deployments: %+v", deploymentNames))

	return deploymentNames, nil
}

func calcGearValues(beacon string, clientset *kubernetes.Clientset) (int64, float64, error) {
	pods, err := clientset.CoreV1().Pods(nameSpace).List(context.TODO(), metav1.ListOptions{LabelSelector: "genteelbeacon=" + beacon})
	if err != nil {
		slog.Error("Eror getting pods for label genteelbeacon=" + beacon)
		return 0, 0, err
	}

	var number int64 = 0
	var sum float64 = 0
	for _, pod := range pods.Items {
		slog.Debug("Querying " + pod.Name + " at IP " + pod.Status.PodIP)
		req, err := http.NewRequest("GET", "http://"+pod.Status.PodIP+":1333/metrics", nil)
		client := &http.Client{Timeout: 3 * time.Second}

		resp, err := client.Do(req)
		if err != nil {
			slog.Warn("Can't reach pod " + pod.Name + ". Error: " + err.Error())
			continue // we just ignore this pod
		}

		defer resp.Body.Close()
		var greaseVal float64
		responseScanner := bufio.NewScanner(resp.Body)
		var line string
		for responseScanner.Scan() {
			line = responseScanner.Text()
			if strings.HasPrefix(line, "genteelbeacon_greasefactor_p") {
				greaseReturn, err := strconv.ParseFloat(strings.TrimSpace(line[len("genteelbeacon_greasefactor_p"):]), 64)
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

func healthCheck(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "{\"status\":\"healthy\"}")
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

	data := map[string]interface{}{
		"kind":       "MetricValueList",
		"apiVersion": "custom.metrics.k8s.io/v1beta1",
		"metadata": map[string]interface{}{
			"selfLink": "/apis/custom.metrics.k8s.io/v1beta1",
		},
		"items": []interface{}{map[string]interface{}{
			"describedObject": map[string]interface{}{
				"kind":       "Service",
				"namespace":  nameSpace,
				"name":       beacon,
				"apiVersion": "v1beta1",
			},
			"metricName": "gearvalue",
			"timestamp":  fmt.Sprintf(time.Now().Format(time.RFC3339)),
			"value":      fmt.Sprintf("%d", int64(math.Round(beaconGear.Average*100))),
		},
		}}

	jsonData, err := json.Marshal(data)
	if err != nil {
		slog.Error("could not marshal json: %s\n", err.Error())
		return
	}

	fmt.Fprint(w, string(jsonData))
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
		beacons, err := getBeacons(clientset)
		if err != nil {
			slog.Error(err.Error())
		}
		for _, element := range beacons {
			number, average, err := calcGearValues(element, clientset)
			if err != nil {
				slog.Error(err.Error())
			}
			gearStats[element] = gearStat{number, average}
			slog.Debug(fmt.Sprintf("Average for "+element+" is: %f\n", (average)))
		}
		time.Sleep(5 * time.Second)
	}
}

func RunGearsmith() {
	gearStats = make(map[string]gearStat)
	ns, err := GetNamespace()
	if err != nil {
		panic(err.Error())
	}
	nameSpace = ns
	slog.Debug("Running in namespace: " + nameSpace)
	// run in background
	go setGearStats()

	router := http.NewServeMux()
	router.HandleFunc("/stats", statsServe)
	router.HandleFunc("/apis/custom.metrics.k8s.io/v1beta1", healthCheck)
	router.HandleFunc("/apis/custom.metrics.k8s.io/v1beta1/namespaces/"+ns+"/services/{beacon}/gearvalue", gearValueServe)

	http.ListenAndServeTLS(":6443", "/cert/tls.crt", "/cert/tls.key", router)
}

// kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/genteelbeacon/services/gildedgateway/gearvalue
