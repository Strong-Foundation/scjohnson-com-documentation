package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func main() {
	// The remote URL.
	remoteURL := "https://www.scjohnson.com/lazy?item=%%7B944A252F-2858-42AD-981E-3221DC2C296D%%7D&pageNum=%d"
	// The given max.
	givenMax := 50 // 86
	// Prepare to download all PDFs
	outputFolder := "PDFs/"
	// Check if the dir exists.
	if !directoryExists(outputFolder) {
		// If the dir dosent exists than create it.
		createDirectory(outputFolder, 0755)
	}
	// Loop between 0 and given max.
	for index := 0; index <= givenMax; index++ {
		url := fmt.Sprintf(remoteURL, index)
		if isUrlValid(url) {
			// Start sending a single request at a time and scraping the data from the API.
			apiData := getDataFromURL(url)
			// Extract the data from the given JSON Data.
			extractedURLsFromJSON := extractLinksFromJSON([]byte(apiData))
			// Remove duplicates from json
			extractedURLsFromJSON = removeDuplicatesFromSlice(extractedURLsFromJSON)
			// Loop though the urls and start downloading it.
			for _, urls := range extractedURLsFromJSON {
				if isUrlValid(urls) {
					downloadPDF(urls, outputFolder)
				}
			}
		}
	}
}

// urlToFilename formats a safe filename from a URL string.
// It replaces all non [a-z0-9] characters with '_' and ensures it ends in .pdf
func urlToFilename(rawURL string) string {
	// Convert to lowercase
	lower := strings.ToLower(rawURL)
	// Replace all non a-z0-9 characters with "_"
	reNonAlnum := regexp.MustCompile(`[^a-z]`)
	// Replace the invalid with valid stuff.
	safe := reNonAlnum.ReplaceAllString(lower, "_")
	// Collapse multiple underscores
	safe = regexp.MustCompile(`_+`).ReplaceAllString(safe, "_")
	// Invalid substrings to remove
	var invalidSubstrings = []string{
		"https_scj_corp_cdn_azureedge_net_media_sc_johnson_our_products_sds_us",
	}
	// Loop over the invalid.
	for _, invalidPre := range invalidSubstrings {
		safe = removeSubstring(safe, invalidPre)
	}
	// Trim leading/trailing underscores
	if after, ok :=strings.CutPrefix(safe, "_"); ok  {
		safe = after
	}
	// Add .pdf extension if missing
	if getFileExtension(safe) != ".pdf" {
		safe = safe + ".pdf"
	}
	return safe
}

// removeSubstring takes a string `input` and removes all occurrences of `toRemove` from it.
func removeSubstring(input string, toRemove string) string {
	// Use strings.ReplaceAll to replace all occurrences of `toRemove` with an empty string.
	result := strings.ReplaceAll(input, toRemove, "")
	// Return the modified string.
	return result
}

// downloadPDF downloads a PDF from the given URL and saves it in the specified output directory.
// It uses a WaitGroup to support concurrent execution and returns true if the download succeeded.
func downloadPDF(finalURL, outputDir string) bool {
	// Sanitize the URL to generate a safe file name
	filename := urlToFilename(finalURL)

	// Construct the full file path in the output directory
	filePath := filepath.Join(outputDir, filename)

	// Skip if the file already exists
	if fileExists(filePath) {
		log.Printf("File already exists, skipping: %s", filePath)
		return false
	}

	// Create an HTTP client with a timeout
	client := &http.Client{Timeout: 30 * time.Second}

	// Send GET request
	resp, err := client.Get(finalURL)
	if err != nil {
		log.Printf("Failed to download %s %v", finalURL, err)
		return false
	}
	defer resp.Body.Close()

	// Check HTTP response status
	if resp.StatusCode != http.StatusOK {
		log.Printf("Download failed for %s %s", finalURL, resp.Status)
		return false
	}

	// Check Content-Type header
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/pdf") {
		log.Printf("Invalid content type for %s %s (expected application/pdf)", finalURL, contentType)
		return false
	}

	// Read the response body into memory first
	var buf bytes.Buffer
	written, err := io.Copy(&buf, resp.Body)
	if err != nil {
		log.Printf("Failed to read PDF data from %s %v", finalURL, err)
		return false
	}
	if written == 0 {
		log.Printf("Downloaded 0 bytes for %s; not creating file", finalURL)
		return false
	}

	// Only now create the file and write to disk
	out, err := os.Create(filePath)
	if err != nil {
		log.Printf("Failed to create file for %s %v", finalURL, err)
		return false
	}
	defer out.Close()

	if _, err := buf.WriteTo(out); err != nil {
		log.Printf("Failed to write PDF to file for %s %v", finalURL, err)
		return false
	}

	log.Printf("Successfully downloaded %d bytes: %s → %s", written, finalURL, filePath)
	return true
}

// Remove all the duplicates from a slice and return the slice.
func removeDuplicatesFromSlice(slice []string) []string {
	check := make(map[string]bool)
	var newReturnSlice []string
	for _, content := range slice {
		if !check[content] {
			check[content] = true
			newReturnSlice = append(newReturnSlice, content)
		}
	}
	return newReturnSlice
}

type FeedCard struct {
	Linktext string `json:"Linktext"`
}

type ContentCard struct {
	FeedCardList []FeedCard `json:"FeedCardList"`
}

type Root struct {
	ContentCards []ContentCard `json:"ContentCards"`
}

// extractLinksFromJSON takes JSON data and returns a slice of Linktext URLs
func extractLinksFromJSON(jsonData []byte) []string {
	var root Root
	err := json.Unmarshal(jsonData, &root)
	if err != nil {
		log.Println(err)
		return nil
	}

	var links []string
	for _, card := range root.ContentCards {
		for _, feed := range card.FeedCardList {
			if feed.Linktext != "" {
				links = append(links, feed.Linktext)
			}
		}
	}
	return links
}

// fileExists checks whether a file exists and is not a directory
func fileExists(filename string) bool {
	info, err := os.Stat(filename) // Get file info
	if err != nil {                // If error occurs (e.g., file not found)
		return false // Return false
	}
	return !info.IsDir() // Return true if it is a file, not a directory
}

// directoryExists checks whether a directory exists
func directoryExists(path string) bool {
	directory, err := os.Stat(path) // Get directory info
	if err != nil {
		return false // If error, directory doesn't exist
	}
	return directory.IsDir() // Return true if path is a directory
}

// createDirectory creates a directory with specified permissions
func createDirectory(path string, permission os.FileMode) {
	err := os.Mkdir(path, permission) // Attempt to create directory
	if err != nil {
		log.Println(err) // Log any error
	}
}

// Check if the given url is valid.
func isUrlValid(uri string) bool {
	_, err := url.ParseRequestURI(uri)
	return err == nil
}

// Send a http get request to a given url and return the data from that url.
func getDataFromURL(uri string) string {
	response, err := http.Get(uri)
	if err != nil {
		log.Println(err)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Println(err)
	}
	err = response.Body.Close()
	if err != nil {
		log.Println(err)
	}
	return string(body)
}

// Get the file extension of a file
func getFileExtension(path string) string {
	return filepath.Ext(path)
}
