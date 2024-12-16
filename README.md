# File-to-Video Encoder/Decoder

A Go-based tool that can encode files into video format and decode them back. It supports both local files and YouTube URLs as input sources.

## Features

- Encode any file into a lossless video format (FFV1)
- Decode videos back to their original files
- Support for processing single files or entire directories
- YouTube video URL support for decoding
- Lossless conversion using all three RGB channels

## Prerequisites

- Go 1.16 or higher
- OpenCV (required for GoCV)
- FFmpeg (for video encoding/decoding)

### Installing Dependencies

**Ubuntu/Debian:**
```
sudo apt-get install libopencv-dev

````

**macOS:**
```
brew install opencv
```

Then install the Go dependencies:
```
go mod tidy
```

## Usage

### Encoding Files to Video
```
go run main.go -e <input_folder> <output_folder>
```


### Decoding Videos Back to Files
```
go run main.go -d <input_folder_or_url> <output_folder>
```

### Examples

Encode a single file:

```
go run main.go -e myfile.txt output/

```
Encode all files in a directory:
```
go run main.go -e input_files/ output_videos/
```

Decode a video:
```
go run main.go -d video.mkv output_files/
```

Decode from YouTube URL (Not working):
```
go run main.go -d "https://youtube.com/watch?v=..." output_files/
```


## Technical Details

- Video Resolution: 640x480
- Frame Rate: 30 FPS
- Codec: FFV1 (lossless)
- Each pixel stores 3 bytes of data (one in each RGB channel)

## How It Works

The tool converts files into video format by:
- Reading the input file as bytes
- Mapping each 3 bytes to a pixel's RGB values
- Creating frames from these pixels
- Encoding frames using the lossless FFV1 codec

When decoding, the process is reversed to reconstruct the original file.
