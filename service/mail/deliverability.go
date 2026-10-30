package mail

import (
	"fmt"
	"net"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"golang.org/x/net/publicsuffix"
)

// DeliverabilityReport is what the installation knows about the domain of the
// sender.
//
// The mail of a new installation lands in the spam folder for one reason: the
// DNS of the domain does not tell the providers that the messages of the
// server are the messages of the domain. The panel of the administrator shows
// the checks below instead of guessing.
type DeliverabilityReport struct {
	Domain string `json:"domain"`
	// Encrypted reports whether the connection to the SMTP server is encrypted.
	Encrypted bool `json:"encrypted"`
	// SPF is the record that allows the server to send the mail of the domain.
	SPF     string `json:"spf"`
	SPFOK   bool   `json:"spf_ok"`
	DMARC   string `json:"dmarc"`
	DMARCOK bool   `json:"dmarc_ok"`
	DKIM    string `json:"dkim"`
	DKIMOK  bool   `json:"dkim_ok"`
	// DKIMSelector is the selector of the key that answered the lookup,
	// empty when no selector of the domain answered. DKIMChecked lists the
	// selectors that were tried, so the administrator sees that the panel
	// looked for the key instead of guessing its absence.
	DKIMSelector string   `json:"dkim_selector"`
	DKIMChecked  []string `json:"dkim_checked"`
	MX           string   `json:"mx"`
	MXOK         bool     `json:"mx_ok"`
	Hints        []Hint   `json:"hints"`
	Readiness    int      `json:"readiness"`
}

// Hint is one advice of the report.
type Hint struct {
	// Level is `ok`, `warn` or `bad`.
	Level string `json:"level"`
	// Message is the text of the advice as the administrator sees it.
	Message string `json:"message"`
}

// CheckDeliverability reads the DNS of the domain of the sender.
//
// The function talks to the resolver of the host, a report is never cached: the
// record of a domain changes while the installation runs and the administrator
// expects the page to show the change.
func CheckDeliverability() DeliverabilityReport {
	from := common.SMTPFrom
	if from == "" {
		from = common.SMTPAccount
	}
	report := DeliverabilityReport{
		Encrypted: common.SMTPSSLEnabled || common.SMTPStartTLSEnabled,
		Hints:     make([]Hint, 0, 6),
	}
	at := strings.LastIndex(from, "@")
	if at < 0 || at == len(from)-1 {
		report.Hints = append(report.Hints, Hint{Level: "bad", Message: "The address of the sender is not a valid address."})
		return report
	}
	domain := strings.ToLower(strings.TrimSpace(from[at+1:]))
	report.Domain = domain

	report.SPF, report.SPFOK = lookupSPF(domain)
	report.DMARC, report.DMARCOK = lookupDMARC(domain)
	report.DKIM, report.DKIMSelector, report.DKIMChecked, report.DKIMOK = lookupDKIM(domain)
	report.MX, report.MXOK = lookupMX(domain)

	checks := 0
	if report.SPFOK {
		checks++
	}
	if report.DMARCOK {
		checks++
	}
	if report.DKIMOK {
		checks++
	}
	if report.MXOK {
		checks++
	}
	report.Readiness = checks * 100 / 4

	if !report.Encrypted {
		report.Hints = append(report.Hints, Hint{
			Level:   "bad",
			Message: "The connection to the mail server is not encrypted. Providers distrust an unencrypted sender and the password of the server travels in clear text. Turn on SSL or STARTTLS.",
		})
	}
	if !report.SPFOK {
		report.Hints = append(report.Hints, Hint{
			Level:   "bad",
			Message: "The domain has no SPF record that allows the mail server. Add a TXT record: v=spf1 include:<the host of your SMTP server> -all",
		})
	}
	if !report.DKIMOK {
		report.Hints = append(report.Hints, Hint{
			Level:   "warn",
			Message: "No DKIM key answered at the common selectors of the domain and of the domain above it. A provider is free to use any selector: read the selector on the DKIM page of your mail server (Poste.io shows it next to the record) and type it into the DKIM selector field of the Email settings, the panel asks the DNS for that name as well.",
		})
	}
	if !report.DMARCOK {
		report.Hints = append(report.Hints, Hint{
			Level:   "bad",
			Message: "The domain has no DMARC record. Add a TXT record on _dmarc: v=DMARC1; p=none; rua=mailto:<your address> and tighten it to quarantine after a week.",
		})
	}
	if !report.MXOK {
		report.Hints = append(report.Hints, Hint{
			Level:   "warn",
			Message: "The domain has no MX record. A reader that answers the message of the installation cannot reach the domain.",
		})
	}
	if report.Readiness == 100 {
		report.Hints = append(report.Hints, Hint{
			Level:   "ok",
			Message: "SPF, DKIM, DMARC and MX are in place. A new domain still needs a warm up: send the first campaigns to a small group of readers.",
		})
	}
	return report
}

// lookupSPF reads the SPF record of a domain.
func lookupSPF(domain string) (string, bool) {
	records, err := lookupTXT(domain)
	if err != nil {
		return "", false
	}
	for _, record := range records {
		trimmed := strings.TrimSpace(record)
		if strings.HasPrefix(strings.ToLower(trimmed), "v=spf1") {
			return trimmed, true
		}
	}
	return "", false
}

// lookupDMARC reads the DMARC record of a domain.
func lookupDMARC(domain string) (string, bool) {
	records, err := lookupTXT("_dmarc." + domain)
	if err != nil {
		return "", false
	}
	for _, record := range records {
		trimmed := strings.TrimSpace(record)
		if strings.HasPrefix(strings.ToLower(trimmed), "v=dmarc1") {
			return trimmed, true
		}
	}
	return "", false
}

// lookupMX reads the domain that receives the answers of the readers.
func lookupMX(domain string) (string, bool) {
	records, err := lookupMXRecords(domain)
	if err != nil || len(records) == 0 {
		return "", false
	}
	hosts := make([]string, 0, len(records))
	for _, record := range records {
		hosts = append(hosts, strings.TrimSuffix(record.Host, "."))
	}
	return strings.Join(hosts, ", "), true
}

// dkimSelectors are the selectors that the panel asks for when the
// administrator did not name one.
//
// The selector of a DKIM key is chosen by the provider of the mail and the
// standard does not publish it, so a lookup is a guess. The list holds the
// selectors of the providers that installations usually run (Poste.io among
// them) and the lookup ends with the selector that the administrator typed.
var dkimSelectors = []string{
	"default",
	"mail",
	"dkim",
	"poste",
	"smtp",
	"email",
	"google",
	"selector1",
	"selector2",
	"k1",
	"s1",
	"s2",
	"smtpapi",
	"mta",
	"host",
	"postfix",
	"key1",
	"dkim1",
}

// lookupDKIM looks for a DKIM key of the domain.
//
// The configured selector is asked first: the administrator that read the
// selector of the provider on the page of the provider ends the guess here.
// The common selectors of the providers are asked after it, the selectors that
// were tried are returned with the answer so that the panel can name them.
//
// The domain of the sender and the domain above it are both asked, because a
// mail server that sends from `mail.example.com` publishes the key of the
// organization at `example.com`, and a panel that reads only the host reports a
// signed domain as unsigned.
func lookupDKIM(domain string) (string, string, []string, bool) {
	selectors := make([]string, 0, len(dkimSelectors)+1)
	configured := strings.TrimSpace(common.OptionMap["DKIMSelector"])
	configured = strings.TrimSpace(strings.TrimSuffix(configured, "._domainkey"))
	if configured != "" {
		selectors = append(selectors, configured)
	}
	for _, selector := range dkimSelectors {
		if selector == configured {
			continue
		}
		selectors = append(selectors, selector)
	}

	for _, candidate := range dkimDomains(domain) {
		for _, selector := range selectors {
			if dkimRecord(selector, candidate) {
				return fmt.Sprintf("%s._domainkey.%s", selector, candidate), selector, selectors, true
			}
		}
	}
	return "", "", selectors, false
}

// dkimDomains returns the names that may carry the key of a domain.
//
// The host itself comes first; the organization above it follows when the host
// is a subdomain and the name above it is not a public suffix, so the lookup of
// `mail.example.co.uk` asks `example.co.uk` and never `co.uk`.
func dkimDomains(domain string) []string {
	parts := strings.Split(domain, ".")
	if len(parts) < 3 {
		return []string{domain}
	}
	parent := strings.Join(parts[1:], ".")
	suffix, _ := publicsuffix.PublicSuffix(domain)
	if parent == suffix {
		return []string{domain}
	}
	return []string{domain, parent}
}

// The DNS lookups of the panel. A test swaps them for a table of the records of
// a domain, so that the rules of the report are tested without a network.
var (
	lookupTXT       = net.LookupTXT
	lookupMXRecords = net.LookupMX
)

// dkimRecord reports whether one selector of the domain publishes a key.
//
// A TXT record of a DKIM key carries the public key in `p=`. The record is read
// as a key even when the `v=DKIM1` tag is missing, because some mail servers
// publish the record without the version tag and the key itself is what the
// receivers verify. A revoked key (`p=` with an empty value) is not a key.
func dkimRecord(selector string, domain string) bool {
	records, err := lookupTXT(selector + "._domainkey." + domain)
	if err != nil {
		return false
	}
	for _, record := range records {
		lowered := strings.ToLower(record)
		if !strings.Contains(lowered, "p=") {
			continue
		}
		for _, part := range strings.Split(lowered, ";") {
			part = strings.TrimSpace(part)
			if !strings.HasPrefix(part, "p=") {
				continue
			}
			if strings.TrimSpace(strings.TrimPrefix(part, "p=")) != "" {
				return true
			}
		}
	}
	return false
}
