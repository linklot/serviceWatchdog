package main

import (
	"io/ioutil"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"os/exec"
	"bytes"
	"strings"
	"net"
	"time"
	"strconv"
	tm "github.com/buger/goterm"
	"fmt"
)

const TimeInterval = 1

type Service struct {
	Dir           string
	Host          string
	Port          int
	Name          string
	Branch        string
	status        string
	lastCheckTime string
}

func main() {
	filename := "services.yml"
	services := readConfig(filename)
	serviceMap := make(map[string]Service)

	monitorChan := make(chan Service)

	for _, service := range services {
		service.Branch = getServiceGitBranch(service)
		service.status = getServiceStatus(service)

		serviceMap[service.Name] = service
	}
	printServicesStatus(services, serviceMap)

	for _, service := range services {
		go watchServiceStatus(monitorChan, service)
	}

	for {
		serviceChecked := <-monitorChan
		statusChanged := isStatusChanged(serviceMap, serviceChecked)
		if statusChanged {
			printServicesStatus(services, serviceMap)
		}
	}
}

func readConfig(filename string) []Service {
	var services []Service

	source, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalf("Config file %s not found", filename)
		os.Exit(0)
	}

	err = yaml.Unmarshal(source, &services)
	if err != nil {
		log.Fatalf("Cannot unmarshal configuration: %v", err)
		os.Exit(0)
	}

	return services
}

func getServiceGitBranch(service Service) string {
	if service.Name == "mongodb" {
		return "-"
	}

	var out bytes.Buffer
	cmd := exec.Command("bash", "-c", "git branch | grep \\*")
	cmd.Dir = service.Dir
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		// Cannot read Git branch
		log.Println(err)
		return ""
	}

	s := out.String()
	result := strings.Replace(s, "* ", "", -1)
	return result
}

func getServiceStatus(service Service) string {
	_, err := net.DialTimeout("tcp", service.Host+":"+strconv.Itoa(service.Port), time.Second)
	if err != nil {
		// Service is down
		return "DOWN"
	}

	return "RUNNING"
}

func watchServiceStatus(monitorChan chan Service, service Service) {
	service.Branch = getServiceGitBranch(service)

	for {
		service.status = getServiceStatus(service)
		service.lastCheckTime = time.Now().Format(time.RFC3339)

		monitorChan <- service
		time.Sleep(TimeInterval * time.Second)
	}
}

func isStatusChanged(serviceMap map[string]Service, service Service) bool {
	serviceName := service.Name
	if existingService, ok := serviceMap[serviceName]; ok {
		// existing service found
		if existingService.status != service.status {
			serviceMap[serviceName] = service
			return true
		}
	}

	serviceMap[serviceName] = service
	return false
}

func printServicesStatus(services []Service, serviceMap map[string]Service) {
	tm.Clear()
	tm.MoveCursor(0, 0)
	totals := tm.NewTable(0, 10, 10, ' ', 0)
	fmt.Fprintf(totals, "%s\t%s\t%s\t%s\n", tm.Color("NAME", tm.BLUE), tm.Color("PORT", tm.BLUE), tm.Color("STATUS", tm.BLUE), tm.Color("BRANCH", tm.BLUE))

	for _, service := range services {
		serviceName := service.Name
		serviceWithStatus := serviceMap[serviceName]

		name := tm.Color(serviceName, tm.WHITE)

		port := tm.Color(strconv.Itoa(serviceWithStatus.Port), tm.YELLOW)

		status := tm.Color("DOWN", tm.RED)
		if serviceWithStatus.status == "RUNNING" {
			status = tm.Color(serviceWithStatus.status, tm.GREEN)
		}

		branch := tm.Color(serviceWithStatus.Branch, tm.WHITE)

		fmt.Fprintf(totals, "%s\t%s\t%s\t%s\n", name, port, status, branch)
	}

	tm.Println(totals)
	tm.Flush()
}
