package mail

import (
	"fmt"
	"net"
	"strings"

	"github.com/QuantumNous/new-api/common"
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
	SPF       string `json:"spf"`
	SPFOK     bool   `json:"spf_ok"`
	DMARC     string `json:"dmarc"`
	DMARCOK   bool   `json:"dmarc_ok"`
	DKIM      string `json:"dkim"`
	DKIMOK    bool   `json:"dkim_ok"`
	MX        string `json:"mx"`
	MXOK      bool   `json:"mx_ok"`
	Hints     []Hint `json:"hints"`
	Readiness int    `json:"readiness"`
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
	report.DKIM, report.DKIMOK = lookupDKIM(domain)
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
			Message: "No DKIM key was found for the domain. Set up the DKIM key that your SMTP provider gives you (a TXT record with the name <selector>._domainkey).",
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
	records, err := net.LookupTXT(domain)
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
	records, err := net.LookupTXT("_dmarc." + domain)
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
	records, err := net.LookupMX(domain)
	if err != nil || len(records) == 0 {
		return "", false
	}
	hosts := make([]string, 0, len(records))
	for _, record := range records {
		hosts = append(hosts, strings.TrimSuffix(record.Host, "."))
	}
	return strings.Join(hosts, ", "), true
}

// lookupDKIM looks for a DKIM key of the domain.
//
// The selector of the key is chosen by the provider of the mail and the
// standard does not publish it, the common selectors of the providers are
// asked. A domain that signs with another selector is reported as unknown and
// the administrator reads the selector of the provider.
func lookupDKIM(domain string) (string, bool) {
	selectors := []string{"default", "google", "selector1", "selector2", "k1", "s1", "s2", "mail", "dkim", "smtp"}
	for _, selector := range selectors {
		records, err := net.LookupTXT(selector + "._domainkey." + domain)
		if err != nil {
			continue
		}
		for _, record := range records {
			if strings.Contains(strings.ToLower(record), "v=dkim1") ||
				strings.Contains(strings.ToLower(record), "k=rsa") ||
				strings.Contains(strings.ToLower(record), "p=") {
				return fmt.Sprintf("%s._domainkey", selector), true
			}
		}
	}
	return "", false
}
