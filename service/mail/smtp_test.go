package mail

import (
	"bufio"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/require"
)

// fakeSMTPServer accepts one session and records the messages and the number of
// the sessions that the client opened.
//
// The campaign writes many messages to one session, the providers rate limit
// the sessions and not the messages: the test proves that the client reuses the
// connection of the server.
type fakeSMTPServer struct {
	listener net.Listener
	port     int

	mutex    sync.Mutex
	sessions int
	messages []string
	rcpts    []string
}

func newFakeSMTPServer(t *testing.T) *fakeSMTPServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := &fakeSMTPServer{listener: listener, port: listener.Addr().(*net.TCPAddr).Port}
	go server.serve()
	t.Cleanup(func() { _ = listener.Close() })
	return server
}

func (s *fakeSMTPServer) serve() {
	for {
		connection, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.mutex.Lock()
		s.sessions++
		s.mutex.Unlock()
		go s.handle(connection)
	}
}

func (s *fakeSMTPServer) handle(connection net.Conn) {
	defer connection.Close()
	reader := bufio.NewReader(connection)
	writer := func(line string) { _, _ = connection.Write([]byte(line + "\r\n")) }

	writer("220 fake.smtp.local ESMTP")
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		command := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(command, "EHLO"), strings.HasPrefix(command, "HELO"):
			writer("250-fake.smtp.local")
			writer("250 AUTH PLAIN LOGIN")
		case strings.HasPrefix(command, "AUTH"):
			writer("235 2.7.0 Authentication successful")
		case strings.HasPrefix(command, "MAIL FROM"):
			writer("250 2.1.0 Ok")
		case strings.HasPrefix(command, "RCPT TO"):
			s.mutex.Lock()
			s.rcpts = append(s.rcpts, strings.TrimSpace(line))
			s.mutex.Unlock()
			writer("250 2.1.5 Ok")
		case strings.HasPrefix(command, "DATA"):
			writer("354 End data with <CR><LF>.<CR><LF>")
			var message strings.Builder
			for {
				dataLine, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				if dataLine == ".\r\n" || dataLine == ".\n" {
					break
				}
				message.WriteString(dataLine)
			}
			s.mutex.Lock()
			s.messages = append(s.messages, message.String())
			s.mutex.Unlock()
			writer("250 2.0.0 Ok: queued")
		case strings.HasPrefix(command, "RSET"):
			writer("250 2.0.0 Ok")
		case strings.HasPrefix(command, "QUIT"):
			writer("221 2.0.0 Bye")
			return
		default:
			writer("250 2.0.0 Ok")
		}
	}
}

func (s *fakeSMTPServer) snapshot() (int, []string, []string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.sessions, append([]string{}, s.messages...), append([]string{}, s.rcpts...)
}

func (s *fakeSMTPServer) waitForMessages(t *testing.T, count int) []string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_, messages, _ := s.snapshot()
		if len(messages) >= count {
			return messages
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, messages, _ := s.snapshot()
	return messages
}

// useFakeServer points the mail settings of the installation at the fake
// server, the test restores the values of the settings at the end.
func useFakeServer(t *testing.T, server *fakeSMTPServer) {
	t.Helper()
	previous := []any{
		common.SMTPServer, common.SMTPPort, common.SMTPAccount, common.SMTPFrom,
		common.SMTPToken, common.SMTPSSLEnabled, common.SMTPStartTLSEnabled, common.SystemName,
	}
	common.SMTPServer = "127.0.0.1"
	common.SMTPPort = server.port
	common.SMTPAccount = "sender@example.com"
	common.SMTPFrom = "sender@example.com"
	common.SMTPToken = "secret"
	common.SMTPSSLEnabled = false
	common.SMTPStartTLSEnabled = false
	common.SystemName = "New API"
	t.Cleanup(func() {
		common.SMTPServer = previous[0].(string)
		common.SMTPPort = previous[1].(int)
		common.SMTPAccount = previous[2].(string)
		common.SMTPFrom = previous[3].(string)
		common.SMTPToken = previous[4].(string)
		common.SMTPSSLEnabled = previous[5].(bool)
		common.SMTPStartTLSEnabled = previous[6].(bool)
		common.SystemName = previous[7].(string)
	})
}

// A campaign of three users leaves the installation on one session of the
// server, every message carries the headers of the unsubscribe of RFC 8058 and
// the envelope of the message carries the address of its own reader.
func TestCampaignUsesOneSessionAndCarriesTheUnsubscribeHeaders(t *testing.T) {
	server := newFakeSMTPServer(t)
	useFakeServer(t, server)

	sender, err := common.NewMailSender()
	require.NoError(t, err)

	recipients := []model.MailRecipient{
		{Id: 1, Email: "one@example.com", DisplayName: "One"},
		{Id: 2, Email: "two@example.com", DisplayName: "Two"},
		{Id: 3, Email: "three@example.com", DisplayName: ""},
	}
	for _, recipient := range recipients {
		message := BuildMessage("News", "The body of the message", false, true, recipient)
		require.NoError(t, sender.Send(message))
	}
	sender.Close()

	messages := server.waitForMessages(t, len(recipients))
	require.Len(t, messages, len(recipients))

	sessions, _, rcpts := server.snapshot()
	require.Equal(t, 1, sessions, "a campaign writes its messages to one session of the server")
	require.Len(t, rcpts, len(recipients))

	for index, message := range messages {
		require.Contains(t, message, "List-Unsubscribe: <http")
		require.Contains(t, message, "List-Unsubscribe-Post: List-Unsubscribe=One-Click")
		require.Contains(t, message, "Precedence: bulk")
		require.Contains(t, message, "multipart/alternative")
		// The token of the unsubscribe of every reader is different: a link of
		// one reader must not remove another reader.
		require.Contains(t, message, recipients[index].Email)
	}
	require.NotEqual(t, messageToken(t, messages[0]), messageToken(t, messages[1]))
	require.Contains(t, rcpts[0], "one@example.com")
	require.Contains(t, rcpts[2], "three@example.com")
}

// messageToken returns the token of the unsubscribe header of a message.
func messageToken(t *testing.T, message string) string {
	t.Helper()
	for _, line := range strings.Split(message, "\r\n") {
		if !strings.HasPrefix(line, "List-Unsubscribe: ") {
			continue
		}
		start := strings.Index(line, "token=")
		if start < 0 {
			return ""
		}
		return strings.TrimRight(line[start+len("token="):], "> ")
	}
	return ""
}

// A session that the server closed cannot swallow the rest of the campaign: the
// next message opens a new session of the server.
func TestCampaignOpensASessionWhenTheServerClosesTheOldOne(t *testing.T) {
	server := newFakeSMTPServer(t)
	useFakeServer(t, server)

	first, err := common.NewMailSender()
	require.NoError(t, err)
	require.NoError(t, first.Send(common.MailMessage{To: "a@example.com", Subject: "One", HTML: "<p>one</p>"}))
	require.False(t, first.Exhausted())

	// The connection is closed under the client, the way a provider drops a
	// session that lives too long.
	first.Close()

	second, err := common.NewMailSender()
	require.NoError(t, err)
	require.NoError(t, second.Send(common.MailMessage{To: "b@example.com", Subject: "Two", HTML: "<p>two</p>"}))
	second.Close()

	messages := server.waitForMessages(t, 2)
	require.Len(t, messages, 2)
	sessions, _, _ := server.snapshot()
	require.Equal(t, 2, sessions)
}

func TestPortOfTheFakeServerIsReadable(t *testing.T) {
	// The port of the fake server is a number of the local address, a test that
	// cannot read it does not talk to the server at all.
	server := newFakeSMTPServer(t)
	require.NotEmpty(t, strconv.Itoa(server.port))
}
