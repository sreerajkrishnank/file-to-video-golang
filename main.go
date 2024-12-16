package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gocv.io/x/gocv"
	"github.com/kkdai/youtube/v2"
	"io"
)

// fileToVideo reads a file and encodes it into a video.
// Each pixel stores 3 bytes (one in each channel: Blue, Green, Red).
func fileToVideo(inputFilename, outputFilename string, width, height, fps int) error {
	data, err := os.ReadFile(inputFilename)
	if err != nil {
		return fmt.Errorf("failed to read input file: %v", err)
	}

	// Each pixel = 3 bytes
	bytesPerPixel := 3
	bytesPerFrame := width * height * bytesPerPixel
	totalFrames := int(math.Ceil(float64(len(data)) / float64(bytesPerFrame)))

	// Pad the data if it doesn't exactly fill the last frame
	requiredBytes := totalFrames * bytesPerFrame
	if len(data) < requiredBytes {
		padded := make([]byte, requiredBytes)
		copy(padded, data)
		data = padded
	}

	// Use a lossless codec (FFV1) to prevent data corruption
	writer, err := gocv.VideoWriterFile(outputFilename, "FFV1", float64(fps), width, height, true)
	if err != nil {
		return fmt.Errorf("failed to create video writer: %v", err)
	}
	defer writer.Close()

	// Prepare a Mat for output frame (3 channels, 8 bits per channel)
	frame := gocv.NewMatWithSize(height, width, gocv.MatTypeCV8UC3)
	defer frame.Close()

	frameData,_ := frame.DataPtrUint8()
	if frameData == nil {
		return fmt.Errorf("failed to get frame data pointer")
	}

	dataIndex := 0
	pixelCount := width * height

	for f := 0; f < totalFrames; f++ {
		frameBytes := data[dataIndex : dataIndex+bytesPerFrame]
		dataIndex += bytesPerFrame

		// For each pixel i:
		// Blue = frameBytes[i*3]
		// Green = frameBytes[i*3+1]
		// Red = frameBytes[i*3+2]
		for i := 0; i < pixelCount; i++ {
			srcOffset := i * 3
			dstOffset := i * 3
			frameData[dstOffset] = frameBytes[srcOffset]     // Blue
			frameData[dstOffset+1] = frameBytes[srcOffset+1] // Green
			frameData[dstOffset+2] = frameBytes[srcOffset+2] // Red
		}

		if err := writer.Write(frame); err != nil {
			return fmt.Errorf("error writing frame %d: %v", f, err)
		}
	}

	return nil
}

// videoToFile decodes a video (either from local file or URL) created by fileToVideo back into a file.
// Reads 3 bytes per pixel (Blue, Green, Red) and reconstructs the original data.
func videoToFile(inputVideo, outputFilename string) error {
	var cap *gocv.VideoCapture
	var err error

	if isURL(inputVideo) {
		// Download YouTube video to a temporary file first
		tempFile, err := downloadYouTubeVideo(inputVideo)
		if err != nil {
			return fmt.Errorf("failed to download YouTube video: %v", err)
		}
		defer os.Remove(tempFile) // Clean up temp file when done
		
		cap, _ = gocv.VideoCaptureFile(tempFile)
	} else {
		cap, err = gocv.VideoCaptureFile(inputVideo)
	}

	if err != nil {
		return fmt.Errorf("failed to open video: %v", err)
	}
	defer cap.Close()

	var allBytes []byte

	frame := gocv.NewMat()
	defer frame.Close()

	for {
		if ok := cap.Read(&frame); !ok || frame.Empty() {
			break
		}

		frameData,_ := frame.DataPtrUint8()
		if frameData == nil {
			return fmt.Errorf("failed to get frame data pointer from decoded frame")
		}

		pixelCount := frame.Rows() * frame.Cols()
		// Extract the 3 bytes per pixel
		for i := 0; i < pixelCount; i++ {
			offset := i * 3
			blueVal := frameData[offset]
			greenVal := frameData[offset+1]
			redVal := frameData[offset+2]
			allBytes = append(allBytes, blueVal, greenVal, redVal)
		}
	}

	// Write the reconstructed bytes to file
	err = os.WriteFile(outputFilename, allBytes, 0644)
	if err != nil {
		return fmt.Errorf("failed to write output file: %v", err)
	}

	return nil
}

// New helper function to download YouTube videos
func downloadYouTubeVideo(url string) (string, error) {
	client := youtube.Client{}
	video, err := client.GetVideo(url)
	if err != nil {
		return "", fmt.Errorf("failed to get video info: %v", err)
	}

	// Try to get the lowest quality format to minimize download size
	// since we only need the video for data extraction
	formats := video.Formats.Quality("144p")
	if len(formats) == 0 {
		// Fallback to any available format
		formats = video.Formats
	}
	if len(formats) == 0 {
		return "", fmt.Errorf("no suitable video formats found")
	}

	// Sort formats by size (ascending) and pick the smallest one
	sort.Slice(formats, func(i, j int) bool {
		return formats[i].ContentLength < formats[j].ContentLength
	})

	// Get the stream
	stream, _, err := client.GetStream(video, &formats[0])
	if err != nil {
		return "", fmt.Errorf("failed to get video stream: %v", err)
	}
	defer stream.Close()

	// Create temporary file
	tempFile, err := os.CreateTemp("", "youtube-*.mp4")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %v", err)
	}
	defer tempFile.Close()

	// Copy the video to the temp file with a buffer
	buf := make([]byte, 1024*1024) // 1MB buffer
	_, err = io.CopyBuffer(tempFile, stream, buf)
	if err != nil {
		os.Remove(tempFile.Name()) // Clean up on error
		return "", fmt.Errorf("failed to download video: %v", err)
	}

	return tempFile.Name(), nil
}

func isURL(path string) bool {
	return strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://")
}

func main() {
	if len(os.Args) != 4 {
		fmt.Println("Usage:")
		fmt.Println("  Encode folder: go run main.go -e <input_folder> <output_folder>")
		fmt.Println("  Decode folder: go run main.go -d <input_folder_or_url> <output_folder>")
		os.Exit(1)
	}

	operation := os.Args[1]
	inputPath := os.Args[2]
	outputPath := os.Args[3]

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		log.Fatalf("Error creating output directory: %v", err)
	}

	fileInfo, err := os.Stat(inputPath)

	switch operation {
	case "-e":
		if err != nil {
			log.Fatalf("Error accessing input path: %v", err)
		}

		if fileInfo.IsDir() {
			// Process directory
			files, err := os.ReadDir(inputPath)
			if err != nil {
				log.Fatalf("Error reading directory: %v", err)
			}

			for _, file := range files {
				if file.IsDir() {
					continue // Skip subdirectories
				}
				inputFile := filepath.Join(inputPath, file.Name())
				outputVideo := filepath.Join(outputPath, file.Name()+".mkv")

				fmt.Printf("Processing: %s\n", inputFile)
				if err := fileToVideo(inputFile, outputVideo, 640, 480, 30); err != nil {
					log.Printf("Error encoding %s: %v", inputFile, err)
					continue
				}
				fmt.Printf("Encoded %s into %s\n", inputFile, outputVideo)
			}
		} else {
			// Process single file
			outputVideo := filepath.Join(outputPath, filepath.Base(inputPath)+".mkv")
			if err := fileToVideo(inputPath, outputVideo, 640, 480, 30); err != nil {
				log.Fatalf("Encoding failed: %v", err)
			}
			fmt.Printf("Encoded %s into %s\n", inputPath, outputVideo)
		}

	case "-d":
		// Decode workflow: handle folder or a single file/URL
		if err != nil && !isURL(inputPath) {
			// If not a URL and stat failed, it's an error
			log.Fatalf("Error accessing input path: %v", err)
		}

		if err == nil && fileInfo.IsDir() {
			// Process directory
			files, err := os.ReadDir(inputPath)
			if err != nil {
				log.Fatalf("Error reading directory: %v", err)
			}

			for _, file := range files {
				if file.IsDir() || !strings.HasSuffix(file.Name(), ".mkv") {
					continue // Skip directories and non-mkv files
				}
				inputVideo := filepath.Join(inputPath, file.Name())
				outputFile := filepath.Join(outputPath, strings.TrimSuffix(filepath.Base(inputVideo), ".mkv")+".decoded")

				fmt.Printf("Processing: %s\n", inputVideo)
				if err := videoToFile(inputVideo, outputFile); err != nil {
					log.Printf("Error decoding %s: %v", inputVideo, err)
					continue
				}
				fmt.Printf("Decoded %s into %s\n", inputVideo, outputFile)
			}
		} else {
			// Process single file or URL
			if isURL(inputPath) {
				// If input is a URL, decode directly from the URL
				outputFile := filepath.Join(outputPath, "youtube.decoded")
				fmt.Printf("Decoding from URL: %s\n", inputPath)
				if err := videoToFile(inputPath, outputFile); err != nil {
					log.Fatalf("Decoding failed from URL %s: %v", inputPath, err)
				}
				fmt.Printf("Decoded video from %s into %s\n", inputPath, outputFile)
			} else {
				// Process single local mkv file
				outputFile := filepath.Join(outputPath, strings.TrimSuffix(filepath.Base(inputPath), ".mkv")+".decoded")
				fmt.Printf("Decoding: %s\n", inputPath)
				if err := videoToFile(inputPath, outputFile); err != nil {
					log.Fatalf("Decoding failed: %v", err)
				}
				fmt.Printf("Decoded %s into %s\n", inputPath, outputFile)
			}
		}

	default:
		fmt.Println("Invalid operation. Use -e for encode or -d for decode")
		os.Exit(1)
	}
}
