package main

import (
	vision "cloud.google.com/go/vision/apiv1"
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func createDir(path string) {
	err := os.Mkdir(path, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
}

func flushToFile(records [][]string) {
	dt := time.Now().Format("2006-01-02 15:04:05")
	filename := "output " + dt + ".csv"
	fmt.Println("Saving data to " + filename)
	f, err := os.Create(filename)
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(f)
	if err != nil {
		log.Fatalln("Failed to open file", err)
	}
	w := csv.NewWriter(f)
	defer w.Flush()
	for _, record := range records {
		if err := w.Write(record); err != nil {
			log.Fatalln("Error writing record to file", err)
		}
	}
}

func main() {
	ctx := context.Background()
	client, err := vision.NewImageAnnotatorClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer func(client *vision.ImageAnnotatorClient) {
		err := client.Close()
		if err != nil {
			log.Fatalf("Failed to create client: %v", err)
		}
	}(client)

	workdir := "./workdir"
	e := os.RemoveAll(workdir)
	if e != nil {
		log.Fatal(e)
	}
	createDir(workdir)

	files, err := os.ReadDir("./videos")
	if err != nil {
		log.Fatal(err)
	}

	videoCount := strconv.Itoa(len(files))
	fmt.Println("Processing " + videoCount + " videos")
	var records = [][]string{
		{"video", "labels"},
	}

	for i, file := range files {
		filename := strings.Split(file.Name(), ".")[0]
		fmt.Println("Processing " + file.Name() + " " + strconv.Itoa(i+1) + "/" + videoCount)
		outPath := workdir + "/" + filename
		createDir(outPath)

		inPath := "./videos/" + file.Name()
		// todo adjust so it works on windows
		command := "ffmpeg -i " + inPath + " -r 0.1 " + outPath + "/output_%03d.jpg"
		fmt.Println("Running command: " + command)
		_, err := exec.Command("bash", "-c", command).Output()
		if err != nil {
			log.Fatal(err)
		}

		images, err := os.ReadDir(outPath)
		if err != nil {
			log.Fatal(err)
		}
		m := map[string]bool{}
		var slice []string
		fmt.Println("Collecting image labels")
		for _, image := range images {
			img := outPath + "/" + image.Name()
			imgContents, err := os.Open(img)
			if err != nil {
				log.Fatalf("Failed to read file: %v", err)
			}
			defer func(imgContents *os.File) {
				err := imgContents.Close()
				if err != nil {
					log.Fatal(err)
				}
			}(imgContents)
			visionImg, err := vision.NewImageFromReader(imgContents)
			if err != nil {
				log.Fatalf("Failed to create image: %v", err)
			}
			labels, err := client.DetectLabels(ctx, visionImg, nil, 50)
			if err != nil {
				log.Fatalf("Failed to detect labels: %v", err)
			}
			for _, label := range labels {
				_, ok := m[label.Description]
				if !ok {
					m[label.Description] = true
					slice = append(slice, label.Description)
				}
			}
		}
		fmt.Println("Finished " + file.Name())
		uniqueLabels := strings.Join(slice[:], ",")
		record := []string{file.Name(), uniqueLabels}
		records = append(records, record)
	}
	flushToFile(records)
}
