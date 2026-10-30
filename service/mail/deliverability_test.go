package mail

import (
	"errors"
	"net"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

// fakeDNS answers the lookups of the report from a table, a missing name is an
// error like the one of a resolver. The table holds TXT records only; a test
// that needs an MX record stubs the lookup itself.
func fakeDNS(t *testing.T, records map[string][]string) {
	t.Helper()
	previousTXT := lookupTXT
	previousMX := lookupMXRecords
	lookupTXT = func(name string) ([]string, error) {
		if values, ok := records[name]; ok {
			return values, nil
		}
		return nil, errors.New("no such host")
	}
	lookupMXRecords = func(name string) ([]*net.MX, error) {
		return nil, errors.New("no such host")
	}
	t.Cleanup(func() {
		lookupTXT = previousTXT
		lookupMXRecords = previousMX
	})
}

func TestDKIMFindsACustomSelector(t *testing.T) {
	// A provider is free to sign with any selector: the key of this domain is
	// published at `poste._domainkey` and the panel used to ask the common
	// selectors only, therefore it reported the domain as unsigned.
	fakeDNS(t, map[string][]string{
		"poste._domainkey.example.com": {"v=DKIM1; k=rsa; p=MIIBIjANBgkq"},
	})

	record, selector, checked, ok := lookupDKIM("example.com")
	require.True(t, ok)
	require.Equal(t, "poste", selector)
	require.Equal(t, "poste._domainkey.example.com", record)
	require.Contains(t, checked, "poste")
}

func TestDKIMUsesTheConfiguredSelectorFirst(t *testing.T) {
	previous := common.OptionMap
	common.OptionMap = map[string]string{"DKIMSelector": "shop-2026._domainkey"}
	t.Cleanup(func() { common.OptionMap = previous })

	fakeDNS(t, map[string][]string{
		"shop-2026._domainkey.example.com": {"k=rsa; p=MIIBIjANBgkq"},
	})

	record, selector, _, ok := lookupDKIM("example.com")
	require.True(t, ok)
	// The administrator typed the name of the record, the panel strips the
	// suffix and asks the DNS for the selector itself.
	require.Equal(t, "shop-2026", selector)
	require.Equal(t, "shop-2026._domainkey.example.com", record)
}

func TestDKIMAcceptsARecordWithoutTheVersionTag(t *testing.T) {
	// Some servers publish the key without `v=DKIM1`. The key is what the
	// receiver verifies, the version tag is not what makes it a key.
	fakeDNS(t, map[string][]string{
		"mail._domainkey.example.com": {"k=rsa; p=MIIBIjANBgkq"},
	})
	_, selector, _, ok := lookupDKIM("example.com")
	require.True(t, ok)
	require.Equal(t, "mail", selector)
}

func TestDKIMLooksAboveTheHostOfTheSender(t *testing.T) {
	// The sender is noreply@mail.example.com and the key of the organization is
	// published at example.com, the name that the panel of the mail server
	// shows. A lookup of the host alone reports the domain as unsigned.
	fakeDNS(t, map[string][]string{
		"mail._domainkey.example.com": {"v=DKIM1; k=rsa; p=MIIBIjANBgkq"},
	})

	record, selector, _, ok := lookupDKIM("mail.example.com")
	require.True(t, ok)
	require.Equal(t, "mail", selector)
	require.Equal(t, "mail._domainkey.example.com", record)
}

func TestDKIMNeverAsksAPublicSuffix(t *testing.T) {
	// example.co.uk is the organization, the registry below it never holds the
	// key of the installation.
	require.Equal(t, []string{"mail.example.co.uk", "example.co.uk"}, dkimDomains("mail.example.co.uk"))
	require.Equal(t, []string{"example.co.uk"}, dkimDomains("example.co.uk"))
	require.Equal(t, []string{"example.com"}, dkimDomains("example.com"))
}

func TestDKIMIgnoresARevokedKey(t *testing.T) {
	// A key with an empty `p=` is revoked: the domain publishes the selector
	// and no key, the report must not call it signed.
	fakeDNS(t, map[string][]string{
		"default._domainkey.example.com": {"v=DKIM1; k=rsa; p="},
	})
	_, _, checked, ok := lookupDKIM("example.com")
	require.False(t, ok)
	require.Contains(t, checked, "default")
}

func TestDeliverabilityReportNamesTheKey(t *testing.T) {
	previousFrom := common.SMTPFrom
	previousOption := common.OptionMap
	common.SMTPFrom = "noreply@example.com"
	common.OptionMap = map[string]string{"DKIMSelector": "s1"}
	t.Cleanup(func() {
		common.SMTPFrom = previousFrom
		common.OptionMap = previousOption
	})

	fakeDNS(t, map[string][]string{
		"example.com":               {"v=spf1 include:example.com -all"},
		"_dmarc.example.com":        {"v=DMARC1; p=none"},
		"s1._domainkey.example.com": {"v=DKIM1; k=rsa; p=MIIBIjANBgkq"},
	})

	report := CheckDeliverability()
	require.Equal(t, "example.com", report.Domain)
	require.True(t, report.SPFOK)
	require.True(t, report.DMARCOK)
	require.True(t, report.DKIMOK)
	require.Equal(t, "s1", report.DKIMSelector)
	require.NotEmpty(t, report.DKIMChecked)
	// MX is not in the table of the fake resolver: the report says so instead
	// of a hundred percent.
	require.False(t, report.MXOK)
	require.Less(t, report.Readiness, 100)
}
