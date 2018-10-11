package walg

import (
	"bytes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/Mynom/wal-g"
	"github.com/Mynom/wal-g/walparser"
	"io"
	"testing"
)

var locations = []walparser.BlockLocation{
	*walparser.NewBlockLocation(1, 2, 3, 4),
	*walparser.NewBlockLocation(5, 6, 7, 8),
}

func TestReadWrite(t *testing.T) {
	var buf bytes.Buffer
	writer := walg.NewBlockLocationWriter(&buf)
	reader := walg.NewBlockLocationReader(&buf)
	for _, location := range locations {
		err := writer.WriteLocation(location)
		assert.NoError(t, err)
	}
	actualLocations := make([]walparser.BlockLocation, 0)
	for {
		location, err := reader.ReadNextLocation()
		if errors.Cause(err) == io.EOF {
			break
		}
		assert.NoError(t, err)
		actualLocations = append(actualLocations, *location)
	}
	assert.Equal(t, locations, actualLocations)
}

func TestWriteLocationsTo(t *testing.T) {
	var buf bytes.Buffer
	err := walg.WriteLocationsTo(&buf, locations)
	assert.NoError(t, err)
	reader := walg.NewBlockLocationReader(&buf)
	actualLocations := make([]walparser.BlockLocation, 0)
	for {
		location, err := reader.ReadNextLocation()
		if errors.Cause(err) == io.EOF {
			break
		}
		assert.NoError(t, err)
		actualLocations = append(actualLocations, *location)
	}
	assert.Equal(t, locations, actualLocations)
}

func TestReadLocationsFrom(t *testing.T) {
	var buf bytes.Buffer
	err := walg.WriteLocationsTo(&buf, locations)
	assert.NoError(t, err)
	actualLocations, err := walg.ReadLocationsFrom(&buf)
	assert.NoError(t, err)
	assert.Equal(t, locations, actualLocations)
}
