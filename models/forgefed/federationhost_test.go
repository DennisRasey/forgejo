// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forgefed

import (
	"strings"
	"testing"
	"time"

	"forgejo.org/modules/validation"

	"github.com/stretchr/testify/require"
)

func Test_FederationHostValidation(t *testing.T) {
	sut := FederationHost{
		HostFqdn: "host.do.main",
		NodeInfo: NodeInfo{
			SoftwareName: "forgejo",
		},
		LatestActivity: time.Now(),
		HostPort:       443,
		HostSchema:     "https",
	}
	if res, err := validation.IsValid(sut); !res {
		t.Errorf("sut should be valid but was %q", err)
	}

	sut = FederationHost{
		HostFqdn: "",
		NodeInfo: NodeInfo{
			SoftwareName: "forgejo",
		},
		LatestActivity: time.Now(),
		HostPort:       443,
		HostSchema:     "https",
	}
	if res, _ := validation.IsValid(sut); res {
		t.Error("sut should be invalid: HostFqdn empty")
	}

	sut = FederationHost{
		HostFqdn: strings.Repeat("fill", 64),
		NodeInfo: NodeInfo{
			SoftwareName: "forgejo",
		},
		LatestActivity: time.Now(),
		HostPort:       443,
		HostSchema:     "https",
	}
	if res, _ := validation.IsValid(sut); res {
		t.Error("sut should be invalid: HostFqdn too long (len=256)")
	}

	sut = FederationHost{
		HostFqdn:       "host.do.main",
		NodeInfo:       NodeInfo{},
		LatestActivity: time.Now(),
		HostPort:       443,
		HostSchema:     "https",
	}
	if res, _ := validation.IsValid(sut); res {
		t.Error("sut should be invalid: NodeInfo invalid")
	}

	sut = FederationHost{
		HostFqdn: "host.do.main",
		NodeInfo: NodeInfo{
			SoftwareName: "forgejo",
		},
		LatestActivity: time.Now().Add(1 * time.Hour),
		HostPort:       443,
		HostSchema:     "https",
	}
	if res, _ := validation.IsValid(sut); res {
		t.Error("sut should be invalid: Future timestamp")
	}

	sut = FederationHost{
		HostFqdn: "hOst.do.main",
		NodeInfo: NodeInfo{
			SoftwareName: "forgejo",
		},
		LatestActivity: time.Now(),
		HostPort:       443,
		HostSchema:     "https",
	}
	if res, _ := validation.IsValid(sut); res {
		t.Error("sut should be invalid: HostFqdn lower case")
	}
}

func Test_FederationHostDefaultPort(t *testing.T) {
	// test default HTTPS port is not in the URI
	sut := FederationHost{
		HostFqdn: "host.do.main",
		NodeInfo: NodeInfo{
			SoftwareName: "forgejo",
		},
		LatestActivity: time.Now(),
		HostPort:       443,
		HostSchema:     "https",
	}
	if res, err := validation.IsValid(sut); !res {
		t.Errorf("sut should be valid but was %q", err)
	}
	url := sut.AsURL()
	require.Equal(t, "https://host.do.main", url.String())

	// test default HTTP port is not in the URI
	sut = FederationHost{
		HostFqdn: "host.do.main",
		NodeInfo: NodeInfo{
			SoftwareName: "forgejo",
		},
		LatestActivity: time.Now(),
		HostPort:       80,
		HostSchema:     "http",
	}
	if res, err := validation.IsValid(sut); !res {
		t.Errorf("sut should be valid but was %q", err)
	}
	url = sut.AsURL()
	require.Equal(t, "http://host.do.main", url.String())

	// test custom ports are present in the URI
	sut = FederationHost{
		HostFqdn: "host.do.main",
		NodeInfo: NodeInfo{
			SoftwareName: "forgejo",
		},
		LatestActivity: time.Now(),
		HostPort:       8443,
		HostSchema:     "https",
	}
	if res, err := validation.IsValid(sut); !res {
		t.Errorf("sut should be valid but was %q", err)
	}
	url = sut.AsURL()
	require.Equal(t, "https://host.do.main:8443", url.String())

	// test zero port (currently invalid, but possible to manually construct)
	sut = FederationHost{
		HostFqdn: "host.do.main",
		NodeInfo: NodeInfo{
			SoftwareName: "forgejo",
		},
		LatestActivity: time.Now(),
		HostPort:       0,
		HostSchema:     "https",
	}
	if res, _ := validation.IsValid(sut); res {
		t.Errorf("sut should be invalid")
	}
	url = sut.AsURL()
	require.Equal(t, "https://host.do.main", url.String())
}
