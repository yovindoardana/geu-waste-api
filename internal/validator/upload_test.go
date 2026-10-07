package validator

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/textproto"
	"testing"
)

func createTestPNGFileHeader(width, height int) (*multipart.FileHeader, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="proof"; filename="test.png"`)
	h.Set("Content-Type", "image/png")

	part, err := writer.CreatePart(h)
	if err != nil {
		return nil, err
	}
	part.Write(buf.Bytes())
	writer.Close()

	reader := multipart.NewReader(body, writer.Boundary())
	form, err := reader.ReadForm(MaxMultipartMem)
	if err != nil {
		return nil, err
	}

	return form.File["proof"][0], nil
}

func TestValidateProofFile(t *testing.T) {
	t.Run("Valid PNG", func(t *testing.T) {
		fh, err := createTestPNGFileHeader(100, 100)
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		ext, err := ValidateProofFile(fh)
		if err != nil {
			t.Fatalf("ValidateProofFile() unexpected error = %v", err)
		}
		if ext != ".png" {
			t.Errorf("expected extension .png, got %s", ext)
		}
	})

	t.Run("Oversized dimensions rejected", func(t *testing.T) {
		fh, err := createTestPNGFileHeader(10001, 1) // width > 10000
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		_, err = ValidateProofFile(fh)
		if err == nil {
			t.Fatal("expected error for oversized width, got nil")
		}
	})
}
