package download

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	// Poseidon
	"github.com/MythicAgents/poseidon/Payload_Type/poseidon/agent_code/pkg/utils/structs"
)

// Test the zipFilesAndDirectories function
func TestZipFilesAndDirectories(t *testing.T) {
	// Setup test files and directories
	testDir := t.TempDir()
	testFile1 := filepath.Join(testDir, "file1.txt")
	testFile2 := filepath.Join(testDir, "file2.txt")
	subDir := filepath.Join(testDir, "subdir")
	subFile := filepath.Join(subDir, "subfile.txt")

	// Create files and subdirectory
	os.WriteFile(testFile1, []byte("This is file1"), 0644)
	os.WriteFile(testFile2, []byte("This is file2"), 0644)
	os.Mkdir(subDir, 0755)
	os.WriteFile(subFile, []byte("This is subfile"), 0644)

	// Test compression
	paths := []string{testFile1, testFile2, subDir}
	zipBuffer, err := zipFilesAndDirectories(paths)
	if err != nil {
		t.Fatalf("Failed to zip files and directories: %s", err)
	}

	// Validate that the zip file is not empty
	if zipBuffer.Len() == 0 {
		t.Error("Zip file is empty")
	}
}

// TestRunWithCompression tests the Run function with compression enabled
func TestRunWithCompression(t *testing.T) {
	// Setup test files
	testDir := t.TempDir()
	testFile := filepath.Join(testDir, "file.txt")
	os.WriteFile(testFile, []byte("This is a test file"), 0644)

	// Create task parameters
	params := zipDownloadArgs{
		Paths:    []string{testFile},
		Compress: true,
	}
	paramsJSON, _ := json.Marshal(params)

	// Create mock job and task
	mockJob := structs.Job{
		SendResponses:       make(chan structs.Response, 10),
		SendFileToMythic:    make(chan structs.SendFileToMythicStruct, 10),
	}
	task := structs.Task{
		Params: string(paramsJSON),
		Job:    &mockJob,
	}

	// Run the function
	go Run(task)

	// Wait for results
	select {
	case msg := <-mockJob.SendResponses:
		if !msg.Completed {
			t.Errorf("Expected task to be completed, but got error: %s", msg.UserOutput)
		}
	case <-mockJob.SendFileToMythic:
		t.Log("File sent to Mythic successfully")
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for task to complete")
	}
}
