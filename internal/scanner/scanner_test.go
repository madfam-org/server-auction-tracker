package scanner

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/madfam-org/server-auction-tracker/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockHTTPClient struct {
	response *http.Response
	err      error
}

func (m *mockHTTPClient) Do(_ *http.Request) (*http.Response, error) {
	return m.response, m.err
}

var testJSON = `{
	"server": [
		{
			"id": 1001,
			"key": 1001,
			"name": "Server Auction",
			"cpu": "AMD Ryzen 5 3600 6-Core Processor",
			"cpu_count": 1,
			"ram_size": 64,
			"hdd_arr": ["512 GB NVMe SSD", "512 GB NVMe SSD"],
			"hdd_count": 2,
			"hdd_size": 512,
			"serverDiskData": {"nvme": [512, 512], "sata": [], "hdd": [], "general": [512]},
			"datacenter": "HEL1-DC7",
			"price": 39,
			"setup_price": 0,
			"specials": ["ECC", "IPv4"],
			"is_ecc": true,
			"traffic": "unlimited",
			"bandwidth": 1000,
			"next_reduce": 0,
			"fixed_price": false
		},
		{
			"id": 1002,
			"key": 1002,
			"name": "Server Auction",
			"cpu": "Intel Core i5-13500",
			"cpu_count": 1,
			"ram_size": 64,
			"hdd_arr": ["1024 GB NVMe SSD", "1024 GB NVMe SSD"],
			"hdd_count": 2,
			"hdd_size": 1024,
			"serverDiskData": {"nvme": [1024, 1024], "sata": [], "hdd": [], "general": [1024]},
			"datacenter": "FSN1-DC14",
			"price": 55,
			"setup_price": 0,
			"specials": ["IPv4"],
			"is_ecc": false,
			"traffic": "unlimited",
			"bandwidth": 1000,
			"next_reduce": 0,
			"fixed_price": false
		},
		{
			"id": 1003,
			"key": 1003,
			"name": "Server Auction",
			"cpu": "Intel Xeon E-2136",
			"cpu_count": 1,
			"ram_size": 32,
			"hdd_arr": ["256 GB SATA SSD"],
			"hdd_count": 1,
			"hdd_size": 256,
			"serverDiskData": {"nvme": [], "sata": [256], "hdd": [], "general": [256]},
			"datacenter": "HEL1-DC2",
			"price": 28,
			"setup_price": 0,
			"specials": [],
			"is_ecc": false,
			"traffic": "unlimited",
			"bandwidth": 1000,
			"next_reduce": 0,
			"fixed_price": false
		}
	]
}`

func TestFetch(t *testing.T) {
	client := &mockHTTPClient{
		response: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(testJSON)),
		},
	}

	sc := NewWithURL("http://test.local/data.json", client)
	servers, err := sc.Fetch()
	require.NoError(t, err)
	assert.Len(t, servers, 3)
	assert.Equal(t, 1001, servers[0].ID)
	assert.Equal(t, "AMD Ryzen 5 3600 6-Core Processor", servers[0].CPU)
	assert.Equal(t, 64, servers[0].RAMSize)
	assert.Equal(t, 2, servers[0].DriveCount)
	assert.Equal(t, 2, servers[0].NVMeCount)
	assert.InDelta(t, 1.0, servers[0].TotalStorageTB, 0.01)
	assert.Equal(t, 6, servers[0].ParsedCores)
}

func TestFilter(t *testing.T) {
	client := &mockHTTPClient{
		response: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(testJSON)),
		},
	}

	sc := NewWithURL("http://test.local/data.json", client)
	servers, err := sc.Fetch()
	require.NoError(t, err)

	filters := config.Filters{
		MinRAMGB:         64,
		MinCPUCores:      6,
		MinDrives:        2,
		MinDriveSizeGB:   512,
		MaxPriceEUR:      90,
		DatacenterPrefix: "HEL1",
	}

	result := sc.Filter(servers, filters)
	assert.Len(t, result, 1)
	assert.Equal(t, 1001, result[0].ID)
}

func TestFilterNoPrefix(t *testing.T) {
	client := &mockHTTPClient{
		response: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(testJSON)),
		},
	}

	sc := NewWithURL("http://test.local/data.json", client)
	servers, err := sc.Fetch()
	require.NoError(t, err)

	filters := config.Filters{
		MinRAMGB:    64,
		MinCPUCores: 6,
		MinDrives:   2,
		MaxPriceEUR: 90,
	}

	result := sc.Filter(servers, filters)
	assert.Len(t, result, 2)
}
