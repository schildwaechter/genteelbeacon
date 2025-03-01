// Genteel Beacon - Grumpy Gearsmith
// Copyright Carsten Thiel 2025
//
// SPDX-Identifier: Apache-2.0

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
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
		logger.Error("Error getting deployments! " + err.Error())
		return nil, err
	}
	var deploymentNames []string

	for _, deploy := range deployments.Items {
		deploymentNames = append(deploymentNames, deploy.Name)
	}

	logger.Debug(fmt.Sprintf("Deployments: %+v", deploymentNames))

	return deploymentNames, nil
}

func calcValues(beacon string, clientset *kubernetes.Clientset) (int64, float64, int64, float64, error) {
	pods, err := clientset.CoreV1().Pods(nameSpace).List(context.TODO(), metav1.ListOptions{LabelSelector: "genteelbeacon=" + beacon})
	if err != nil {
		logger.Error("Eror getting pods for label genteelbeacon=" + beacon)
		return 0, 0, 0, 0, err
	}

	var gearNumber int64 = 0
	var inkNumber int64 = 0
	var gearSum float64 = 0
	var inkSum float64 = 0
	for _, pod := range pods.Items {
		logger.Debug("Querying " + pod.Name + " at IP " + pod.Status.PodIP)
		req, err := http.NewRequest("GET", "http://"+pod.Status.PodIP+":1333/metrics", nil)
		client := &http.Client{Timeout: 3 * time.Second}

		resp, err := client.Do(req)
		if err != nil {
			logger.Warn("Can't reach pod " + pod.Name + ". Error: " + err.Error())
			continue // we just ignore this pod
		}

		defer resp.Body.Close()
		var greaseVal float64 = 0
		var inkVal float64 = 0
		responseScanner := bufio.NewScanner(resp.Body)
		var line string
		for responseScanner.Scan() {
			line = responseScanner.Text()
			if strings.HasPrefix(line, "genteelbeacon_greasebuildup_p") {
				greaseReturn, err := strconv.ParseFloat(strings.TrimSpace(line[len("genteelbeacon_greasebuildup_p"):]), 64)
				if err != nil {
					logger.Error("Error parsing prometheus value")
				} else {
					greaseVal = greaseReturn
					gearNumber += 1
					gearSum += greaseVal
				}
			}
			if strings.HasPrefix(line, "genteelbeacon_inkdepletion_p") {
				inkReturn, err := strconv.ParseFloat(strings.TrimSpace(line[len("genteelbeacon_inkdepletion_p"):]), 64)
				if err != nil {
					logger.Error("Error parsing prometheus value")
				} else {
					inkVal = inkReturn
					inkNumber += 1
					inkSum += inkVal
				}
			}
		}
		logger.Debug(fmt.Sprintf("Grease Buildup for "+pod.Name+" is %f\n", (greaseVal)) + fmt.Sprintf("Ink Depletion for "+pod.Name+" is %f\n", (inkVal)))
	}

	return gearNumber, gearSum, inkNumber, inkSum, nil
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
		logger.Warn("No beacon in query!")
	}
	beaconGear := gearStats[beacon]
	if beaconGear.Count == 0 {
		logger.Warn("Queried non-existent gear: " + beacon)
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
			"value":      fmt.Sprintf("%d", int64(math.Round(beaconGear.Sum))),
		},
		}}

	jsonData, err := json.Marshal(data)
	if err != nil {
		logger.Error("could not marshal json: %s\n", err.Error())
		return
	}

	fmt.Fprint(w, string(jsonData))
}

func inkValueServe(w http.ResponseWriter, r *http.Request) {
	beacon := r.PathValue("beacon")
	if beacon == "" {
		logger.Warn("No beacon in query!")
	}
	beaconInk := inkStats[beacon]
	if beaconInk.Count == 0 {
		logger.Warn("Queried non-existent ink: " + beacon)
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
			"metricName": "inkvalue",
			"timestamp":  fmt.Sprintf(time.Now().Format(time.RFC3339)),
			"value":      fmt.Sprintf("%d", int64(math.Round(beaconInk.Sum))),
		},
		}}

	jsonData, err := json.Marshal(data)
	if err != nil {
		logger.Error("could not marshal json: %s\n", err.Error())
		return
	}

	fmt.Fprint(w, string(jsonData))
}

type gearStat struct {
	Count   int64
	Sum     float64
	Average float64
}
type inkStat struct {
	Count   int64
	Sum     float64
	Average float64
}

var gearStats map[string]gearStat
var inkStats map[string]inkStat

func setStats() {
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
			logger.Error(err.Error())
		}
		for _, element := range beacons {
			gearNumber, gearSum, inkNumber, inkSum, err := calcValues(element, clientset)
			if err != nil {
				logger.Error(err.Error())
			}
			var gearAverage float64 = 0
			var inkAverage float64 = 0

			if gearNumber != 0 {
				gearAverage = gearSum / float64(gearNumber)
			}
			if inkNumber != 0 {
				inkAverage = inkSum / float64(inkNumber)
			}
			gearStats[element] = gearStat{gearNumber, gearSum, gearAverage}
			inkStats[element] = inkStat{inkNumber, inkSum, inkAverage}
			logger.Debug(fmt.Sprintf("Gear average for "+element+" is: %f\n", (gearAverage)))
			logger.Debug(fmt.Sprintf("Ink average for "+element+" is: %f\n", (inkAverage)))
		}
		time.Sleep(5 * time.Second)
	}
}

func RunGearsmith() {
	gearStats = make(map[string]gearStat)
	inkStats = make(map[string]inkStat)

	ns, err := GetNamespace()
	if err != nil {
		panic(err.Error())
	}
	nameSpace = ns
	logger.Debug("Running in namespace: " + nameSpace)
	// run in background
	go setStats()

	router := http.NewServeMux()
	router.HandleFunc("/stats", statsServe)
	router.HandleFunc("/apis/custom.metrics.k8s.io/v1beta1", healthCheck)
	router.HandleFunc("/apis/custom.metrics.k8s.io/v1beta1/namespaces/"+ns+"/services/{beacon}/gearvalue", gearValueServe)
	router.HandleFunc("/apis/custom.metrics.k8s.io/v1beta1/namespaces/"+ns+"/services/{beacon}/inkvalue", inkValueServe)

	http.ListenAndServeTLS(":6443", "/cert/tls.crt", "/cert/tls.key", router)
}

// kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/genteelbeacon/services/velvettimepiece/gearvalue
// kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/genteelbeacon/services/gaslightparlour/inkvalue
