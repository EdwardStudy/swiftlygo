package slo

import (
	"fmt"
	"github.com/ncw/swift"
	"regexp"
	"strconv"
)

type Inventory struct {
	uploadNeeded       []bool
	numberUploadNeeded uint
	manifest           *Manifest
	connection         *swift.Connection
	overwrite          bool
	ready              bool
}

func NewInventory(manifest *Manifest, connection *swift.Connection, overwrite bool) *Inventory {
	return &Inventory{
		uploadNeeded:       make([]bool, manifest.NumberChunks),
		numberUploadNeeded: 0,
		ready:              false,
		manifest:           manifest,
		connection:         connection,
		overwrite:          overwrite,
	}
}

// TakeInventory readies the inventory for use. After this, the ShouldUpload method will
// return whether a given chunk needs upload again.
func (i *Inventory) TakeInventory() error {
	if i.overwrite {
		i.markAll()
		return nil
	}
	containerFiles, err := i.connection.ObjectNamesAll(i.manifest.ContainerName, nil)
	if err != nil {
		return fmt.Errorf("Unable to fetch container names: %s", err)
	}
	fileNameRegex, err := regexp.Compile(i.manifest.GetChunkNameRegex())
	if err != nil {
		return fmt.Errorf("Unable to compile regex to search existing file names: %s", err)
	}
	numberFilesAlreadyUploaded := 0
	for _, name := range containerFiles {
		// Ignoring error because it's possible that files are not part of
		// the current SLO and will not match the naming convention
		numberString := fileNameRegex.FindStringSubmatch(name)
		if numberString == nil || len(numberString) < 2 {
			continue
		}
		number, err := strconv.Atoi(numberString[1])
		if err != nil {
			continue
		}
		i.uploadNeeded[number] = false
		numberFilesAlreadyUploaded++
	}
	i.numberUploadNeeded -= uint(numberFilesAlreadyUploaded)
	fmt.Printf(
		"%d chunks need uploading. Additionally, manifest file is always re-uploaded.\n",
		i.numberUploadNeeded)
	i.ready = true
	return nil
}

// markAll marks all chunks as needing upload.
func (i *Inventory) markAll() {
	for k := range i.uploadNeeded {
		i.uploadNeeded[k] = true
	}
	i.numberUploadNeeded = uint(len(i.uploadNeeded))
	i.ready = true
}

// UploadsNeeded returns how many chunks need to be uploaded. Will panic if called before
// TakeInventory().
func (i *Inventory) UploadsNeeded() uint {
	if !i.ready {
		panic(fmt.Errorf("UploadsNeeded() called before TakeInventory() on %t", i))
	}
	return i.numberUploadNeeded
}

// ShouldUpload returns whether the chunkNumber needs to be uploaded. Will panic if
// called before TakeInventory or if an invalid chunkNumber is provided.
func (i *Inventory) ShouldUpload(chunkNumber uint) bool {
	if !i.ready {
		panic(fmt.Errorf("ShouldUpload() called before TakeInventory() on %t", i))
	} else if chunkNumber >= uint(len(i.uploadNeeded)) {
		panic(fmt.Errorf("ShouldUpload() called with invalid chunkNumber %d, (only %d chunks)", chunkNumber, len(i.uploadNeeded)))
	}
	return i.uploadNeeded[chunkNumber]
}