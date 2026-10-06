// Package tagwriter provides audio metadata tag writing for MP3 (ID3v2.3) and FLAC (Vorbis Comment + Picture).
// Zero external dependencies.
package tagwriter

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

// TagData holds metadata to embed into an audio file.
type TagData struct {
	Title    string
	Artist   string
	Album    string
	Year     string
	Track    string
	Genre    string
	Comment  string
	Lyrics   string
	CoverJPEG []byte // raw JPEG bytes
}

// WriteTags detects format by extension and writes tags.
func WriteTags(filePath string, data TagData) error {
	lower := strings.ToLower(filePath)
	if strings.HasSuffix(lower, ".mp3") {
		return writeMP3Tags(filePath, data)
	}
	if strings.HasSuffix(lower, ".flac") {
		return writeFLACTags(filePath, data)
	}
	return fmt.Errorf("unsupported format for tag writing: %s", filePath)
}

// ──────────────────────────────────────────────
// MP3 ID3v2.3 Tag Writer
// ──────────────────────────────────────────────

func writeMP3Tags(filePath string, data TagData) error {
	// Build ID3v2.3 tag
	tagBytes := buildID3v23(data)
	if tagBytes == nil {
		return nil
	}

	// Read original file
	orig, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Remove existing ID3v2 tag if present (starts with "ID3")
	audioStart := 0
	if len(orig) >= 3 && string(orig[0:3]) == "ID3" {
		// Parse tag size (4 bytes syncsafe at offset 6)
		if len(orig) >= 10 {
			size := int(orig[6])<<21 | int(orig[7])<<14 | int(orig[8])<<7 | int(orig[9])
			audioStart = 10 + size
		}
	}

	// Also remove ID3v1 tag if present (last 128 bytes start with "TAG")
	audioEnd := len(orig)
	if len(orig) >= 128 && string(orig[audioEnd-128:audioEnd-125]) == "TAG" {
		audioEnd -= 128
	}

	// Write new file: new tag + audio data
	out := make([]byte, 0, len(tagBytes)+(audioEnd-audioStart))
	out = append(out, tagBytes...)
	out = append(out, orig[audioStart:audioEnd]...)

	return os.WriteFile(filePath, out, 0644)
}

func buildID3v23(data TagData) []byte {
	var frames []byte

	// TIT2 - Title
	if data.Title != "" {
		frames = append(frames, buildTextFrame("TIT2", data.Title)...)
	}
	// TPE1 - Artist
	if data.Artist != "" {
		frames = append(frames, buildTextFrame("TPE1", data.Artist)...)
	}
	// TALB - Album
	if data.Album != "" {
		frames = append(frames, buildTextFrame("TALB", data.Album)...)
	}
	// TYER - Year
	if data.Year != "" {
		frames = append(frames, buildTextFrame("TYER", data.Year)...)
	}
	// TRCK - Track
	if data.Track != "" {
		frames = append(frames, buildTextFrame("TRCK", data.Track)...)
	}
	// TCON - Genre
	if data.Genre != "" {
		frames = append(frames, buildTextFrame("TCON", data.Genre)...)
	}
	// COMM - Comment
	if data.Comment != "" {
		frames = append(frames, buildCommentFrame(data.Comment)...)
	}
	// USLT - Unsynchronized Lyrics
	if data.Lyrics != "" {
		frames = append(frames, buildLyricsFrame(data.Lyrics)...)
	}
	// APIC - Cover Picture
	if len(data.CoverJPEG) > 0 {
		frames = append(frames, buildCoverFrame(data.CoverJPEG)...)
	}

	if len(frames) == 0 {
		return nil
	}

	// ID3v2.3 header: "ID3" + version(03 00) + flags(00) + size (4 bytes syncsafe)
	size := len(frames)
	header := make([]byte, 10)
	copy(header[0:3], "ID3")
	header[3] = 0x03
	header[4] = 0x00
	header[5] = 0x00
	header[6] = byte((size >> 21) & 0x7F)
	header[7] = byte((size >> 14) & 0x7F)
	header[8] = byte((size >> 7) & 0x7F)
	header[9] = byte(size & 0x7F)

	result := make([]byte, 0, 10+size)
	result = append(result, header...)
	result = append(result, frames...)
	return result
}

func buildTextFrame(frameID, text string) []byte {
	// Encoding byte (01 = UTF-16 with BOM) + BOM + UTF-16LE text
	encoded := encodeUTF16LE(text)
	content := make([]byte, 0, 1+len(encoded))
	content = append(content, 0x01) // UTF-16
	content = append(content, 0xFF, 0xFE) // BOM
	content = append(content, encoded...)

	// Frame header: 4 bytes ID + 4 bytes size (big-endian) + 2 bytes flags
	frame := make([]byte, 10+len(content))
	copy(frame[0:4], frameID)
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(content)))
	frame[8] = 0x00
	frame[9] = 0x00
	copy(frame[10:], content)
	return frame
}

func buildCommentFrame(comment string) []byte {
	// COMM frame: encoding(1) + language(3) + short content desc(terminated) + text
	encoded := encodeUTF16LE(comment)
	content := make([]byte, 0, 1+3+2+len(encoded))
	content = append(content, 0x01)       // UTF-16
	content = append(content, 'e', 'n', 'g') // language
	content = append(content, 0xFF, 0xFE) // BOM for desc (empty)
	content = append(content, 0x00, 0x00) // empty desc terminator
	content = append(content, 0xFF, 0xFE) // BOM for text
	content = append(content, encoded...)

	frame := make([]byte, 10+len(content))
	copy(frame[0:4], "COMM")
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(content)))
	copy(frame[10:], content)
	return frame
}

func buildLyricsFrame(lyrics string) []byte {
	// USLT frame: encoding(1) + language(3) + content descriptor(terminated) + lyrics text
	encoded := encodeUTF16LE(lyrics)
	content := make([]byte, 0, 1+3+2+len(encoded))
	content = append(content, 0x01)       // UTF-16
	content = append(content, 'e', 'n', 'g') // language
	content = append(content, 0xFF, 0xFE) // BOM
	content = append(content, 0x00, 0x00) // empty descriptor terminator
	content = append(content, encoded...)

	frame := make([]byte, 10+len(content))
	copy(frame[0:4], "USLT")
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(content)))
	copy(frame[10:], content)
	return frame
}

func buildCoverFrame(imageData []byte) []byte {
	// APIC frame: encoding(1) + mime type(terminated) + picture type(1) + description(terminated) + picture data
	mime := detectImageMime(imageData)
	content := make([]byte, 0, 1+len(mime)+1+1+1+len(imageData))
	content = append(content, 0x00)              // encoding = ISO-8859-1
	content = append(content, []byte(mime)...)    // MIME type
	content = append(content, 0x00)               // terminator
	content = append(content, 0x03)               // picture type = 03 (front cover)
	content = append(content, 0x00)               // empty description (ISO-8859-1)
	content = append(content, imageData...)        // picture data

	frame := make([]byte, 10+len(content))
	copy(frame[0:4], "APIC")
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(content)))
	copy(frame[10:], content)
	return frame
}

func detectImageMime(data []byte) string {
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}
	if len(data) >= 8 && data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "image/png"
	}
	return "image/jpeg"
}

func encodeUTF16LE(s string) []byte {
	runes := []rune(s)
	buf := make([]byte, 0, len(runes)*2)
	for _, r := range runes {
		if r <= 0xFFFF {
			buf = append(buf, byte(r&0xFF), byte((r>>8)&0xFF))
		} else {
			// Surrogate pair
			r -= 0x10000
			high := 0xD800 + (r >> 10)
			low := 0xDC00 + (r & 0x3FF)
			buf = append(buf, byte(high&0xFF), byte((high>>8)&0xFF))
			buf = append(buf, byte(low&0xFF), byte((low>>8)&0xFF))
		}
	}
	return buf
}

// ──────────────────────────────────────────────
// FLAC Metadata Block Writer (Vorbis Comment + Picture)
// ──────────────────────────────────────────────

func writeFLACTags(filePath string, data TagData) error {
	orig, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Verify FLAC marker
	if len(orig) < 4 || string(orig[0:4]) != "fLaC" {
		return fmt.Errorf("not a valid FLAC file")
	}

	// Parse existing metadata blocks
	type metaBlock struct {
		blockType byte
		data      []byte
	}
	var blocks []metaBlock
	offset := 4
	hasAudio := false

	for offset < len(orig) {
		if offset+4 > len(orig) {
			break
		}
		header := orig[offset]
		isLast := header&0x80 != 0
		blockType := header & 0x7F
		blockSize := int(orig[offset+1])<<16 | int(orig[offset+2])<<8 | int(orig[offset+3])

		if blockType == 127 { // invalid
			break
		}

		dataStart := offset + 4
		dataEnd := dataStart + blockSize
		if dataEnd > len(orig) {
			break
		}

		blockData := make([]byte, blockSize)
		copy(blockData, orig[dataStart:dataEnd])

		// Skip existing VORBIS_COMMENT (4) and PICTURE (6) blocks - we'll replace them
		if blockType != 4 && blockType != 6 {
			blocks = append(blocks, metaBlock{blockType: blockType, data: blockData})
		}

		offset = dataEnd
		if isLast {
			hasAudio = true
			break
		}
	}

	if !hasAudio {
		return fmt.Errorf("could not find end of FLAC metadata")
	}

	audioData := orig[offset:]

	// Build new VORBIS_COMMENT block
	vcBlock := buildVorbisComment(data)
	blocks = append(blocks, metaBlock{blockType: 4, data: vcBlock})

	// Build PICTURE block if cover available
	if len(data.CoverJPEG) > 0 {
		picBlock := buildFLACPicture(data.CoverJPEG)
		blocks = append(blocks, metaBlock{blockType: 6, data: picBlock})
	}

	// Reassemble FLAC file
	var out bytes.Buffer
	out.Write([]byte("fLaC"))

	for i, b := range blocks {
		isLast := byte(0)
		if i == len(blocks)-1 {
			isLast = 0x80
		}
		// Block header: 1 byte (last flag + type) + 3 bytes size
		hdr := make([]byte, 4)
		hdr[0] = isLast | b.blockType
		hdr[1] = byte((len(b.data) >> 16) & 0xFF)
		hdr[2] = byte((len(b.data) >> 8) & 0xFF)
		hdr[3] = byte(len(b.data) & 0xFF)
		out.Write(hdr)
		out.Write(b.data)
	}

	out.Write(audioData)

	return os.WriteFile(filePath, out.Bytes(), 0644)
}

func buildVorbisComment(data TagData) []byte {
	var comments []string

	if data.Title != "" {
		comments = append(comments, "TITLE="+data.Title)
	}
	if data.Artist != "" {
		comments = append(comments, "ARTIST="+data.Artist)
	}
	if data.Album != "" {
		comments = append(comments, "ALBUM="+data.Album)
	}
	if data.Year != "" {
		comments = append(comments, "DATE="+data.Year)
	}
	if data.Track != "" {
		comments = append(comments, "TRACKNUMBER="+data.Track)
	}
	if data.Genre != "" {
		comments = append(comments, "GENRE="+data.Genre)
	}
	if data.Comment != "" {
		comments = append(comments, "COMMENT="+data.Comment)
	}
	if data.Lyrics != "" {
		comments = append(comments, "LYRICS="+data.Lyrics)
	}

	var buf bytes.Buffer
	// Vendor string length + vendor string
	vendor := "fn-lx-player"
	binary.Write(&buf, binary.LittleEndian, uint32(len(vendor)))
	buf.WriteString(vendor)

	// Number of comments
	binary.Write(&buf, binary.LittleEndian, uint32(len(comments)))

	// Each comment: length (uint32 LE) + comment string
	for _, c := range comments {
		binary.Write(&buf, binary.LittleEndian, uint32(len(c)))
		buf.WriteString(c)
	}

	return buf.Bytes()
}

func buildFLACPicture(imageData []byte) []byte {
	var buf bytes.Buffer
	// Picture type: 3 = front cover
	binary.Write(&buf, binary.BigEndian, uint32(3))
	// MIME type length + MIME type
	mime := detectImageMime(imageData)
	binary.Write(&buf, binary.BigEndian, uint32(len(mime)))
	buf.WriteString(mime)
	// Description length + description (empty)
	binary.Write(&buf, binary.BigEndian, uint32(0))
	// Width, Height, Color depth, Colors used (all 0 = not specified)
	binary.Write(&buf, binary.BigEndian, uint32(0))
	binary.Write(&buf, binary.BigEndian, uint32(0))
	binary.Write(&buf, binary.BigEndian, uint32(0))
	binary.Write(&buf, binary.BigEndian, uint32(0))
	// Picture data length + data
	binary.Write(&buf, binary.BigEndian, uint32(len(imageData)))
	buf.Write(imageData)
	return buf.Bytes()
}
