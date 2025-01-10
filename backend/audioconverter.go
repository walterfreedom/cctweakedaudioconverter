package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"golang.org/x/net/websocket"

	ffmpeg "github.com/u2takey/ffmpeg-go"
)

func main() {
	http.HandleFunc("/upload", handleUpload)
	http.Handle("/echo", websocket.Handler(EchoServer)) // Associates the WebSocket handler with the "/echo" endpoint.
	fmt.Println("Starting server on :8080...")
	err := http.ListenAndServe("0.0.0.0:8080", nil)
	if err != nil {
		panic(err)
	}

}
func EchoServer(ws *websocket.Conn) {
	defer ws.Close() // Ensure the connection is closed
	var msg string

	for {
		// Receive a message from the client
		err := websocket.Message.Receive(ws, &msg)
		if err != nil {
			fmt.Println("Error receiving message:", err)
			break
		}

		fmt.Println("Message received:", msg)

		// Echo the message back to the client
		err = websocket.Message.Send(ws, msg)
		if err != nil {
			fmt.Println("Error sending message:", err)
			break
		}
	}
}

func allowCORS(w http.ResponseWriter) {

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "*")
}
func handleUpload(w http.ResponseWriter, r *http.Request) {

	if r.Method == http.MethodOptions {
		allowCORS(w)
		// Respond with a 200 OK for preflight request
		w.WriteHeader(http.StatusOK)
		return
	}

	// Handle regular POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	allowCORS(w)

	r.ParseMultipartForm(10 << 20) // Limit upload size to 10 MB
	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error retrieving the file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	fmt.Printf("Uploaded File: %s\n", handler.Filename)

	valid, err := isValidAudioFile(file)
	if err != nil || !valid {
		http.Error(w, "Invalid or unsupported audio file", http.StatusBadRequest)
		return
	}

	// Save the uploaded file to a temporary location
	tempFile, err := os.CreateTemp("./", "upload-*.wav")
	if err != nil {
		http.Error(w, "Could not create temporary file", http.StatusInternalServerError)
		return
	}
	defer tempFile.Close()

	_, err = io.Copy(tempFile, file)
	if err != nil {
		http.Error(w, "Could not save uploaded file", http.StatusInternalServerError)
		return
	}

	outputFile := "./converted.dfpwm"
	err = convertAudiotoMctweaked(tempFile.Name(), outputFile)
	if err != nil {
		http.Error(w, "Error converting file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=converted.dfpwm")
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, outputFile)
}
func convertAudiotoMctweaked(input string, output string) error {
	err := ffmpeg.Input(input).
		Output(output, ffmpeg.KwArgs{
			"ac":  1,
			"c:a": "dfpwm",
			"ar":  "48k",
		}).
		OverWriteOutput().
		Run()
	if err != nil {
		return err
	}
	return nil
}

func isValidAudioFile(file io.Reader) (bool, error) {
	// Check MIME type
	buf := make([]byte, 512)
	_, err := file.Read(buf)
	if err != nil {
		return false, err
	}

	contentType := http.DetectContentType(buf)
	if !strings.HasPrefix(contentType, "audio/") {
		return false, nil
	}

	if seeker, ok := file.(io.Seeker); ok {
		seeker.Seek(0, io.SeekStart)
	}

	return true, nil
}
