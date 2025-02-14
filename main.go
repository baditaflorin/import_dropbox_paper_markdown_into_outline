package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Collection represents a collection in Outline.
type Collection struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// CreateDocumentResponse represents the response from /api/documents.create.
type CreateDocumentResponse struct {
	Data struct {
		Id string `json:"id"`
	} `json:"data"`
	Ok bool `json:"ok"`
}

// CollectionsResponse represents the response from /api/collections.list.
type CollectionsResponse struct {
	Data []Collection `json:"data"`
	Ok   bool         `json:"ok"`
}

var (
	debug     bool
	folderMap = make(map[string]string) // maps relative folder path to Outline document ID
)

// importMarkdownFile uploads a Markdown file to Outline using /api/documents.import.
// The file is imported with the given parentDocumentId (if provided).
func importMarkdownFile(filePath, collectionId, parentDocumentId, host, token string) error {
	// Open the file.
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add the file field.
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("creating form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("copying file content: %w", err)
	}

	// Add required fields.
	if err := writer.WriteField("collectionId", collectionId); err != nil {
		return fmt.Errorf("writing collectionId field: %w", err)
	}
	// Only add parentDocumentId if it's not empty.
	if parentDocumentId != "" {
		if err := writer.WriteField("parentDocumentId", parentDocumentId); err != nil {
			return fmt.Errorf("writing parentDocumentId field: %w", err)
		}
	}
	if err := writer.WriteField("template", "false"); err != nil {
		return fmt.Errorf("writing template field: %w", err)
	}
	if err := writer.WriteField("publish", "true"); err != nil {
		return fmt.Errorf("writing publish field: %w", err)
	}

	// Finalize the multipart form.
	if err := writer.Close(); err != nil {
		return fmt.Errorf("closing writer: %w", err)
	}

	url := host + "/api/documents.import"
	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return fmt.Errorf("creating HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	if debug {
		log.Printf("Importing file: %s (parent: %s) to %s", filePath, parentDocumentId, url)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("executing HTTP request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to import %s: %s", filePath, string(body))
	}

	if debug {
		log.Printf("Imported file: %s, response: %s", filePath, string(body))
	}
	return nil
}

// createFolderDocument creates a "folder" document in Outline using /api/documents.create.
// The folder is represented as a document with a title (folderName) and empty text.
// Only include parentDocumentId if it's not empty.
func createFolderDocument(folderName, collectionId, parentDocumentId, host, token string) (string, error) {
	url := host + "/api/documents.create"

	// Build payload. Only add parentDocumentId if provided.
	payload := map[string]interface{}{
		"collectionId": collectionId,
		"title":        folderName,
		"text":         "",
		"template":     false,
		"publish":      false,
	}
	if parentDocumentId != "" {
		payload["parentDocumentId"] = parentDocumentId
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshalling payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	if debug {
		log.Printf("Creating folder document: %s (parent: %s) via %s", folderName, parentDocumentId, url)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to create folder %s: %s", folderName, string(respBytes))
	}

	var createResp CreateDocumentResponse
	if err := json.Unmarshal(respBytes, &createResp); err != nil {
		return "", fmt.Errorf("unmarshalling response: %w", err)
	}
	if !createResp.Ok {
		return "", fmt.Errorf("failed to create folder %s: %s", folderName, string(respBytes))
	}
	if debug {
		log.Printf("Created folder '%s' with ID: %s", folderName, createResp.Data.Id)
	}
	return createResp.Data.Id, nil
}

// getOrCreateFolder returns the Outline document ID for the given relative folder path.
// It will create folder documents for each segment as needed.
func getOrCreateFolder(relPath, collectionId, host, token string) (string, error) {
	// If the relative path is empty (or "."), then no parent.
	if relPath == "" || relPath == "." {
		return "", nil
	}

	// Normalize using forward slashes.
	relPath = filepath.ToSlash(relPath)

	// If already created, return the stored ID.
	if id, ok := folderMap[relPath]; ok {
		return id, nil
	}

	// Split the path into segments and ensure each folder exists.
	segments := strings.Split(relPath, "/")
	var currentPath string
	var parentID string // for the current segment (empty for top-level)
	for _, seg := range segments {
		if currentPath == "" {
			currentPath = seg
		} else {
			currentPath = currentPath + "/" + seg
		}
		if id, exists := folderMap[currentPath]; exists {
			parentID = id
			continue
		}
		newID, err := createFolderDocument(seg, collectionId, parentID, host, token)
		if err != nil {
			return "", fmt.Errorf("creating folder '%s': %w", currentPath, err)
		}
		folderMap[currentPath] = newID
		parentID = newID
	}
	return parentID, nil
}

// listCollections calls /api/collections.list and prints available collections.
func listCollections(host, token string) error {
	url := host + "/api/collections.list"
	payload := map[string]interface{}{
		"offset": 0,
		"limit":  100,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	if debug {
		log.Printf("Listing collections via %s", url)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to list collections: %s", string(respBytes))
	}

	var collectionsResp CollectionsResponse
	if err := json.Unmarshal(respBytes, &collectionsResp); err != nil {
		return fmt.Errorf("unmarshalling response: %w", err)
	}

	if !collectionsResp.Ok {
		return fmt.Errorf("collections list not OK: %s", string(respBytes))
	}

	fmt.Println("Collections:")
	for _, col := range collectionsResp.Data {
		fmt.Printf("ID: %s, Name: %s, Description: %s\n", col.Id, col.Name, col.Description)
	}
	return nil
}

func main() {
	// Command-line flags.
	folderPtr := flag.String("folder", "output_paper_markdown", "Folder containing Markdown files")
	hostPtr := flag.String("host", "https://app.getoutline.com", "Outline host URL")
	collectionPtr := flag.String("collection", "", "Valid collection UUID to import documents into")
	tokenPtr := flag.String("token", "", "Outline API token")
	listFlag := flag.Bool("list", false, "List collections and exit")
	flag.BoolVar(&debug, "debug", false, "Enable debug logging")
	flag.Parse()

	// Use token from flag or environment.
	token := *tokenPtr
	if token == "" {
		token = os.Getenv("OUTLINE_API_TOKEN")
		if token == "" {
			log.Fatal("Outline API token must be provided via -token flag or OUTLINE_API_TOKEN environment variable")
		}
	}

	// If -list is specified, list collections and exit.
	if *listFlag {
		if err := listCollections(*hostPtr, token); err != nil {
			log.Fatalf("Error listing collections: %v", err)
		}
		return
	}

	// Ensure a valid collection UUID is provided.
	if *collectionPtr == "" {
		log.Fatal("A valid collection UUID must be provided via the -collection flag, or use -list to view collections")
	}

	// Walk the base folder recursively.
	err := filepath.Walk(*folderPtr, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			if debug {
				log.Printf("Processing Markdown file: %s", path)
			}
			// Compute the file's relative path with respect to the base folder.
			relPath, err := filepath.Rel(*folderPtr, path)
			if err != nil {
				return err
			}
			// Get the directory part.
			dir := filepath.Dir(relPath)
			var parentID string
			if dir != "." {
				// Create (or retrieve) folder document(s) for this directory.
				parentID, err = getOrCreateFolder(dir, *collectionPtr, *hostPtr, token)
				if err != nil {
					log.Printf("Error creating folder for %s: %v", dir, err)
					// Proceed with no parent if folder creation fails.
				}
			}
			// Import the Markdown file with the determined parent document ID.
			if err := importMarkdownFile(path, *collectionPtr, parentID, *hostPtr, token); err != nil {
				log.Printf("Error importing file %s: %v", path, err)
			}
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Error walking folder: %v", err)
	}
}
