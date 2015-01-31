package main

import (
	"flag"
	"fmt"
	"log"
)

var (
	configPath string
	config     *Config
	// current state of services we know about
	services ServiceList
	// listen to this channel for update triggers
	updateChan chan string
)

func init() {
	flag.StringVar(&configPath, "conf", "", "config json file")
	flag.Parse()
}

func main() {
	if configPath == "" {
		log.Fatal("You need to pass a -conf")
	}
	config, err := LoadConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Loaded config: %s\n", config)
	tc := NewTaskClient(config)
	services = config.Services

	log.Printf("Loading tasks from marathon\n")
	err = services.LoadAllAppTasks(tc)
	if err != nil {
		log.Fatal(err.Error())
	}

	// just print out what apps and tasks we found
	for _, service := range services {
		log.Printf("Found app %s with %d tasks\n", service.AppId, service.Vhost, len(*service.Tasks))
		for _, task := range *service.Tasks {
			for _, port := range task.Ports {
				log.Printf("  %s:%d\n", task.Host, port)
			}
		}

	}

	// spit out the first config before we start listening for events
	log.Printf("Templating %s with %d services\n", config.TemplateFile, len(services))
	output, err := Template(services, config.TemplateFile)
	if err != nil {
		log.Fatal(err.Error())
	}
	log.Printf("Got output:\n%s\n", output)
	log.Printf("Writing config to %s\n", config.TargetFile)
	err = WriteConfig(output, config.TargetFile)
	if err != nil {
		log.Fatal(err.Error())
	}

	updateChan := make(chan string, 1)

	go func(updateChan *chan string) {
		for {
			message := <-*updateChan
			log.Printf("Got message %s; sleeping %d seconds before emitting config\n", message, config.TemplateDelay)
			//TODO debounce updates
			//time.Sleep(config.TemplateDelay * time.Second)

			log.Printf("Loading tasks from marathon\n")
			err = services.LoadAllAppTasks(tc)
			if err != nil {
				log.Printf("Unable to load tasks from marathon: %s\n", err.Error())
			}

			log.Printf("Templating %s with %d services\n", config.TemplateFile, len(services))
			output, err := Template(services, config.TemplateFile)
			if err != nil {
				log.Printf("Unable to compile template: %s\n", err.Error())
			}

			log.Printf("Writing config to %s\n", config.TargetFile)
			err = WriteConfig(output, config.TargetFile)
			if err != nil {
				log.Printf("Unable to write config to %s: %s\n", config.TargetFile, err.Error())
			}
		}
	}(&updateChan)

	var listenAddr = fmt.Sprintf(":%d", config.HttpPort)
	log.Printf("Listening for events on %s\n", listenAddr)
	log.Fatal(ListenForEvents(listenAddr))

}
