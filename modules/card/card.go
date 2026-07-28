// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package card

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	_ "image/gif"  // for processing gif images
	_ "image/jpeg" // for processing jpeg images
	_ "image/png"  // for processing png images

	"forgejo.org/modules/avatar"
	"forgejo.org/modules/log"
	"forgejo.org/modules/proxy"
	"forgejo.org/modules/setting"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"

	_ "golang.org/x/image/webp" // for processing webp images
)

type Card struct {
	Img    *image.RGBA
	Font   *truetype.Font
	Margin int
	Width  int
	Height int
}

var fontCache = sync.OnceValues(func() (*truetype.Font, error) {
	return truetype.Parse(goregular.TTF)
})

// DefaultSize returns the default size for a card
func DefaultSize() (int, int) {
	return 1200, 600
}

// NewCard creates a new card with the given dimensions in pixels
func NewCard(width, height int) (*Card, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)

	font, err := fontCache()
	if err != nil {
		return nil, err
	}

	return &Card{
		Img:    img,
		Font:   font,
		Margin: 0,
		Width:  width,
		Height: height,
	}, nil
}

// Split splits the card horizontally or vertically by a given percentage; the first card returned has the percentage
// size, and the second card has the remainder.  Both cards draw to a subsection of the same image buffer.
func (c *Card) Split(vertical bool, percentage int) (*Card, *Card) {
	bounds := c.Img.Bounds()
	bounds = image.Rect(bounds.Min.X+c.Margin, bounds.Min.Y+c.Margin, bounds.Max.X-c.Margin, bounds.Max.Y-c.Margin)
	if vertical {
		mid := (bounds.Dx() * percentage / 100) + bounds.Min.X
		subleft := c.Img.SubImage(image.Rect(bounds.Min.X, bounds.Min.Y, mid, bounds.Max.Y)).(*image.RGBA)
		subright := c.Img.SubImage(image.Rect(mid, bounds.Min.Y, bounds.Max.X, bounds.Max.Y)).(*image.RGBA)
		return &Card{Img: subleft, Font: c.Font, Width: subleft.Bounds().Dx(), Height: subleft.Bounds().Dy()},
			&Card{Img: subright, Font: c.Font, Width: subright.Bounds().Dx(), Height: subright.Bounds().Dy()}
	}
	mid := (bounds.Dy() * percentage / 100) + bounds.Min.Y
	subtop := c.Img.SubImage(image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Max.X, mid)).(*image.RGBA)
	subbottom := c.Img.SubImage(image.Rect(bounds.Min.X, mid, bounds.Max.X, bounds.Max.Y)).(*image.RGBA)
	return &Card{Img: subtop, Font: c.Font, Width: subtop.Bounds().Dx(), Height: subtop.Bounds().Dy()},
		&Card{Img: subbottom, Font: c.Font, Width: subbottom.Bounds().Dx(), Height: subbottom.Bounds().Dy()}
}

// SetMargin sets the margins for the card
func (c *Card) SetMargin(margin int) {
	c.Margin = margin
}

type (
	VAlign int64
	HAlign int64
)

const (
	Top VAlign = iota
	Middle
	Bottom
)

const (
	Left HAlign = iota
	Center
	Right
)

// DrawText draws text within the card, respecting margins and alignment
func (c *Card) DrawText(text string, textColor color.Color, sizePt float64, valign VAlign, halign HAlign) ([]string, error) {
	ft := freetype.NewContext()
	ft.SetDPI(72)
	ft.SetFont(c.Font)
	ft.SetFontSize(sizePt)
	ft.SetClip(c.Img.Bounds())
	ft.SetDst(c.Img)
	ft.SetSrc(image.NewUniform(textColor))

	face := truetype.NewFace(c.Font, &truetype.Options{Size: sizePt, DPI: 72})
	fontHeight := ft.PointToFixed(sizePt).Ceil()

	bounds := c.Img.Bounds()
	bounds = image.Rect(bounds.Min.X+c.Margin, bounds.Min.Y+c.Margin, bounds.Max.X-c.Margin, bounds.Max.Y-c.Margin)
	boxWidth, boxHeight := bounds.Size().X, bounds.Size().Y
	// draw.Draw(c.Img, bounds, image.NewUniform(color.Gray{128}), image.Point{}, draw.Src) // Debug draw box

	// Try to apply wrapping to this text; we'll find the most text that will fit into one line, record that line, move
	// on.  We precalculate each line before drawing so that we can support valign="middle" correctly which requires
	// knowing the total height, which is related to how many lines we'll have.
	lines := make([]string, 0)
	textWords := strings.Split(text, " ")
	currentLine := ""
	heightTotal := 0

	for {
		if len(textWords) == 0 {
			// Ran out of words.
			if currentLine != "" {
				heightTotal += fontHeight
				lines = append(lines, currentLine)
			}
			break
		}

		nextWord := textWords[0]
		proposedLine := currentLine
		if proposedLine != "" {
			proposedLine += " "
		}
		proposedLine += nextWord

		proposedLineWidth := font.MeasureString(face, proposedLine)
		if proposedLineWidth.Ceil() > boxWidth {
			// no, proposed line is too big; we'll use the last "currentLine"
			heightTotal += fontHeight
			if currentLine != "" {
				lines = append(lines, currentLine)
				currentLine = ""
				// leave nextWord in textWords and keep going
			} else {
				// just nextWord by itself doesn't fit on a line; well, we can't skip it, but we'll consume it
				// regardless as a line by itself.  It will be clipped by the drawing routine.
				lines = append(lines, nextWord)
				textWords = textWords[1:]
			}
		} else {
			// yes, it will fit
			currentLine = proposedLine
			textWords = textWords[1:]
		}
	}

	textY := 0
	switch valign {
	case Top:
		textY = fontHeight
	case Bottom:
		textY = boxHeight - heightTotal + fontHeight
	case Middle:
		textY = ((boxHeight - heightTotal) / 2) + fontHeight
	}

	for _, line := range lines {
		lineWidth := font.MeasureString(face, line)

		textX := 0
		switch halign {
		case Left:
			textX = 0
		case Right:
			textX = boxWidth - lineWidth.Ceil()
		case Center:
			textX = (boxWidth - lineWidth.Ceil()) / 2
		}

		pt := freetype.Pt(bounds.Min.X+textX, bounds.Min.Y+textY)
		_, err := ft.DrawString(line, pt)
		if err != nil {
			return nil, err
		}

		textY += fontHeight
	}

	return lines, nil
}

// DrawImage fills the card with an image, scaled to maintain the original aspect ratio and centered with respect to the non-filled dimension
func (c *Card) DrawImage(img image.Image) {
	bounds := c.Img.Bounds()
	targetRect := image.Rect(bounds.Min.X+c.Margin, bounds.Min.Y+c.Margin, bounds.Max.X-c.Margin, bounds.Max.Y-c.Margin)
	srcBounds := img.Bounds()
	srcAspect := float64(srcBounds.Dx()) / float64(srcBounds.Dy())
	targetAspect := float64(targetRect.Dx()) / float64(targetRect.Dy())

	var scale float64
	if srcAspect > targetAspect {
		// Image is wider than target, scale by width
		scale = float64(targetRect.Dx()) / float64(srcBounds.Dx())
	} else {
		// Image is taller or equal, scale by height
		scale = float64(targetRect.Dy()) / float64(srcBounds.Dy())
	}

	newWidth := int(math.Round(float64(srcBounds.Dx()) * scale))
	newHeight := int(math.Round(float64(srcBounds.Dy()) * scale))

	// Center the image within the target rectangle
	offsetX := (targetRect.Dx() - newWidth) / 2
	offsetY := (targetRect.Dy() - newHeight) / 2

	scaledRect := image.Rect(targetRect.Min.X+offsetX, targetRect.Min.Y+offsetY, targetRect.Min.X+offsetX+newWidth, targetRect.Min.Y+offsetY+newHeight)
	draw.CatmullRom.Scale(c.Img, scaledRect, img, srcBounds, draw.Over, nil)
}

func fallbackImage() image.Image {
	// can't usage image.Uniform(color.White) because it's infinitely sized causing a panic in the scaler in DrawImage
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.White)
	return img
}

func (c *Card) fetchExternalImage(url string) (image.Image, bool) {
	// Use a short timeout; in the event of any failure we'll be logging and returning a placeholder, but we don't want
	// this rendering process to be slowed down
	client := &http.Client{
		Timeout: 1 * time.Second,
		Transport: &http.Transport{
			Proxy: proxy.Proxy(),
		},
	}

	// FetchExternalImageData expects a absolute URL, so we must change a relative to an absolute one
	if !strings.Contains(url, "://") {
		url = fmt.Sprintf("%s%s", setting.AppURL, strings.TrimPrefix(url, "/"))
	}
	bodyBuffer, err := avatar.FetchExternalImageData(url, client)
	if err != nil {
		return nil, false
	}
	img, _, err := image.Decode(bodyBuffer)
	if err != nil {
		log.Warn("error when decoding external image from %s: %w", url, err)
		return nil, false
	}

	return img, true
}

func (c *Card) DrawExternalImage(url string) {
	image, ok := c.fetchExternalImage(url)
	if !ok {
		image = fallbackImage()
	}
	c.DrawImage(image)
}

// DrawRect draws a rect with the given color
func (c *Card) DrawRect(startX, startY, endX, endY int, color color.Color) {
	draw.Draw(c.Img, image.Rect(startX, startY, endX, endY), &image.Uniform{color}, image.Point{}, draw.Src)
}
