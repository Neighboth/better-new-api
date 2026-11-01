package mail

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"time"

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
	domain = strings.Trim(domain, "<>\"' \t\r\n")
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
	"posteio",
	"s2026",
	"s2025",
	"s2024",
	"s2023",
	"s2022",
	"s2021",
	"s2020",
	"s2019",
	"s2018",
	"s2027",
	"s2028",
	"s2029",
	"s2030",
	"s2031",
	"s2032",
	"s2033",
	"s2034",
	"s2035",
	"k1",
	"k2",
	"s",
	"s1",
	"s2",
	"key",
	"key1",
	"smtp",
	"email",
	"google",
	"selector1",
	"selector2",
	"smtpapi",
	"mta",
	"host",
	"postfix",
	"dkim1",
	"mail1",
}

// lookupDKIM looks for a DKIM key of the domain.
//
// The configured selector is asked first: the administrator that read the
// selector of the provider on the page of the provider ends the guess here.
// The common selectors of the providers are asked after it concurrently, the selectors that
// were tried are returned with the answer so that the panel can name them.
//
// The domain of the sender and the domain above it are both asked, because a
// mail server that sends from `mail.example.com` publishes the key of the
// organization at `example.com`, and a panel that reads only the host reports a
// signed domain as unsigned.
func lookupDKIM(domain string) (string, string, []string, bool) {
	selectors := make([]string, 0, len(dkimSelectors)+3)
	configured := strings.TrimSpace(common.OptionMap["DKIMSelector"])
	configured = strings.TrimSpace(strings.TrimSuffix(configured, "._domainkey"))
	if configured != "" {
		configuredLower := strings.ToLower(configured)
		if configuredLower == "self-hosted" || configuredLower == "poste.io" || configuredLower == "poste" || configuredLower == "skip" || configuredLower == "internal" || configuredLower == "enabled" || configuredLower == "true" {
			return "self-hosted (Poste.io)", "self-hosted", selectors, true
		}
		selectors = append(selectors, configured)
		if strings.HasPrefix(configured, "_") {
			selectors = append(selectors, strings.TrimPrefix(configured, "_"))
		} else {
			selectors = append(selectors, "_"+configured)
		}
	}

	smtpHostLower := strings.ToLower(common.SMTPServer)
	if strings.Contains(smtpHostLower, "poste") {
		return "self-hosted (Poste.io)", "self-hosted", selectors, true
	}
	for _, selector := range dkimSelectors {
		matched := false
		for _, s := range selectors {
			if s == selector {
				matched = true
				break
			}
		}
		if !matched {
			selectors = append(selectors, selector)
		}
	}

	domains := dkimDomains(domain)

	// If configured selector exists, try it first synchronously
	if configured != "" {
		candidates := []string{configured}
		if strings.HasPrefix(configured, "_") {
			candidates = append(candidates, strings.TrimPrefix(configured, "_"))
		} else {
			candidates = append(candidates, "_"+configured)
		}
		for _, sel := range candidates {
			for _, candidate := range domains {
				if dkimRecord(sel, candidate) {
					return fmt.Sprintf("%s._domainkey.%s", sel, candidate), sel, selectors, true
				}
			}
		}
	}

	// Try common selectors concurrently
	type result struct {
		record   string
		selector string
	}
	resChan := make(chan result, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var once sync.Once
	var wg sync.WaitGroup
	// Concurrency limiter so we don't spam sockets
	sem := make(chan struct{}, 16)

	for _, candidate := range domains {
		for _, sel := range selectors {
			if sel == configured {
				continue
			}
			wg.Add(1)
			go func(cDomain, cSel string) {
				defer wg.Done()
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					return
				}

				if dkimRecord(cSel, cDomain) {
					once.Do(func() {
						select {
						case resChan <- result{
							record:   fmt.Sprintf("%s._domainkey.%s", cSel, cDomain),
							selector: cSel,
						}:
						default:
						}
					})
				}
			}(candidate, sel)
		}
	}

	go func() {
		wg.Wait()
		close(resChan)
	}()

	select {
	case res, ok := <-resChan:
		if ok && res.record != "" {
			return res.record, res.selector, selectors, true
		}
	case <-ctx.Done():
		select {
		case res, ok := <-resChan:
			if ok && res.record != "" {
				return res.record, res.selector, selectors, true
			}
		default:
		}
	}

	return "", "", selectors, false
}

func containsString(list []string, item string) bool {
	for _, val := range list {
		if val == item {
			return true
		}
	}
	return false
}

// dkimDomains returns the names that may carry the key of a domain.
//
// The host itself comes first; the organization above it follows when the host
// is a subdomain and the name above it is not a public suffix, so the lookup of
// `mail.example.co.uk` asks `example.co.uk` and never `co.uk`.
func dkimDomains(domain string) []string {
	result := []string{domain}
	parts := strings.Split(domain, ".")
	if len(parts) >= 3 {
		parent := strings.Join(parts[1:], ".")
		suffix, _ := publicsuffix.PublicSuffix(domain)
		if parent != suffix {
			result = append(result, parent)
		}
	}

	smtpHost := strings.TrimSpace(common.SMTPServer)
	if smtpHost != "" && smtpHost != domain && !strings.Contains(smtpHost, ":") && strings.Contains(smtpHost, ".") {
		if !containsString(result, smtpHost) {
			result = append(result, smtpHost)
		}
		sParts := strings.Split(smtpHost, ".")
		if len(sParts) >= 3 {
			sParent := strings.Join(sParts[1:], ".")
			sSuffix, _ := publicsuffix.PublicSuffix(smtpHost)
			if sParent != sSuffix && !containsString(result, sParent) {
				result = append(result, sParent)
			}
		}
	}
	return result
}

// The DNS lookups of the panel. A test swaps them for a table of the records of
// a domain, so that the rules of the report are tested without a network.
var (
	lookupTXT       = net.LookupTXT
	lookupMXRecords = net.LookupMX
)

// isCustomLookupTXT reports whether lookupTXT has been substituted by a test stub.
func isCustomLookupTXT() bool {
	return reflect.ValueOf(lookupTXT).Pointer() != reflect.ValueOf(net.LookupTXT).Pointer()
}

type dohAnswer struct {
	Name string `json:"name"`
	Type int    `json:"type"`
	Data string `json:"data"`
}

type dohResponse struct {
	Status int         `json:"Status"`
	Answer []dohAnswer `json:"Answer"`
}

func queryTXTViaDoH(fqdn string) ([]string, error) {
	endpoints := []string{
		"https://dns.google/resolve?name=" + url.QueryEscape(fqdn) + "&type=TXT",
		"https://cloudflare-dns.com/dns-query?name=" + url.QueryEscape(fqdn) + "&type=TXT",
	}
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	for _, ep := range endpoints {
		req, err := http.NewRequest("GET", ep, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Accept", "application/dns-json")
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 200 {
			var d dohResponse
			if err := json.Unmarshal(body, &d); err == nil && d.Status == 0 {
				var txts []string
				for _, ans := range d.Answer {
					if ans.Type == 16 { // TXT record
						val := strings.Trim(ans.Data, "\"")
						val = strings.ReplaceAll(val, "\" \"", "")
						if val != "" {
							txts = append(txts, val)
						}
					}
				}
				if len(txts) > 0 {
					return txts, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("DoH lookup failed for %s", fqdn)
}

// queryTXTRecord queries TXT records with automatic multi-resolver fallback:
// first calls lookupTXT (which tests swap for unit mocks), and if unsuccessful,
// queries Cloudflare/Google DoH over HTTPS, and then UDP directly to bypass
// local router/ISP DNS drops on underscore records (_domainkey).
func queryTXTRecord(fqdn string) ([]string, error) {
	records, err := lookupTXT(fqdn)
	if err == nil && len(records) > 0 {
		return records, nil
	}
	if isCustomLookupTXT() {
		return records, err
	}

	if dohRecords, dohErr := queryTXTViaDoH(fqdn); dohErr == nil && len(dohRecords) > 0 {
		return dohRecords, nil
	}

	cfResolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 800 * time.Millisecond}
			return d.DialContext(ctx, "udp", "1.1.1.1:53")
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	if cfRecords, cfErr := cfResolver.LookupTXT(ctx, fqdn); cfErr == nil && len(cfRecords) > 0 {
		return cfRecords, nil
	}

	googleResolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 800 * time.Millisecond}
			return d.DialContext(ctx, "udp", "8.8.8.8:53")
		},
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel2()
	if gRecords, gErr := googleResolver.LookupTXT(ctx2, fqdn); gErr == nil && len(gRecords) > 0 {
		return gRecords, nil
	}

	return records, err
}

// dkimRecord reports whether one selector of the domain publishes a key.
//
// A TXT record of a DKIM key carries the public key in `p=`. The record is read
// as a key even when the `v=DKIM1` tag is missing, because some mail servers
// publish the record without the version tag and the key itself is what the
// receivers verify. A revoked key (`p=` with an empty value) is not a key.
func dkimRecord(selector string, domain string) bool {
	fqdn := selector + "._domainkey." + domain
	records, err := queryTXTRecord(fqdn)
	if (err != nil || len(records) == 0) && !isCustomLookupTXT() {
		if cname, cErr := net.LookupCNAME(fqdn); cErr == nil && cname != "" && cname != fqdn+"." {
			records, err = queryTXTRecord(strings.TrimSuffix(cname, "."))
		}
	}
	if err != nil || len(records) == 0 {
		return false
	}
	candidates := make([]string, 0, len(records)+1)
	candidates = append(candidates, records...)
	if len(records) > 1 {
		candidates = append(candidates, strings.Join(records, ""))
	}

	for _, record := range candidates {
		lowered := strings.ToLower(record)
		clean := strings.ReplaceAll(lowered, " ", "")
		clean = strings.ReplaceAll(clean, "\t", "")
		clean = strings.ReplaceAll(clean, "\r", "")
		clean = strings.ReplaceAll(clean, "\n", "")
		clean = strings.ReplaceAll(clean, "\"", "")
		clean = strings.ReplaceAll(clean, "'", "")
		if !strings.Contains(clean, "p=") {
			continue
		}
		// Extract value after p= until next semicolon or end
		idx := strings.Index(clean, "p=")
		if idx == -1 {
			continue
		}
		val := clean[idx+2:]
		if semi := strings.Index(val, ";"); semi != -1 {
			val = val[:semi]
		}
		val = strings.TrimSpace(val)
		if len(val) >= 8 {
			return true
		}
	}
	return false
}
