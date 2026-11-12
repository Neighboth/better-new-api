package middleware

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// The Auto Model: a virtual model named "auto".
//
// A request that names it is rewritten to the real model the router chose, so
// everything downstream (channel selection, price, logs, fallback) works on the
// model that actually answers. The client keeps asking for "auto"; the answer
// is billed as the model that produced it.

// autoModelDecisionKey carries the decision of the router through the request.
const autoModelDecisionKey = service.AutoModelDecisionContextKey

// resolveAutoModelRequest rewrites an "auto" request to a real model.
//
// It returns whether the request asked for the virtual model, so the caller can
// accept a token that is limited to "auto" even though the request now names
// the chosen model.
func resolveAutoModelRequest(c *gin.Context, modelRequest *ModelRequest) (bool, error) {
	if modelRequest == nil || !service.IsAutoModelRequest(modelRequest.Model) {
		return false, nil
	}
	if !service.AutoModelEnabled() {
		return true, fmt.Errorf("the auto model is disabled by the administrator")
	}

	body, err := readReusableBody(c)
	if err != nil {
		return true, err
	}

	usingGroup := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	groups := []string{usingGroup}
	if usingGroup == "" {
		groups = []string{userGroup}
	}
	if usingGroup == "auto" {
		groups = service.GetRequestAutoGroups(c, userGroup)
	}
	candidates := service.GetGroupsEnabledModels(groups)

	blockedModels := service.AutoModelBlockedModels(c.GetInt("id"))
	text, hasImage, hasAudio := autoModelRequestProfile(body, c.Request.URL.Path)
	opts := autoModelRequestOptions(body)

	selection, err := service.ResolveAutoModel(candidates, blockedModels, text, hasImage, hasAudio, opts)
	if err != nil {
		return true, err
	}

	rewritten, contentType, err := rewriteRequestModel(c, body, selection.Model)
	if err != nil {
		return true, err
	}
	if err := replaceReusableBody(c, rewritten); err != nil {
		return true, err
	}
	if contentType != "" {
		c.Request.Header.Set("Content-Type", contentType)
	}

	modelRequest.Model = selection.Model
	c.Set(autoModelDecisionKey, selection)
	return true, nil
}

// autoRequestParamKeys are the auto-model request parameters the gateway
// consumes. They are stripped from the body before it goes upstream: a
// strict provider would reject them, and they are meaningless there.
var autoRequestParamKeys = []string{
	"auto_budget",
	"auto_budget_tokens",
	"auto_price_min",
	"auto_price_max",
	"auto_preference",
	"auto_prefer",
	"auto_instruction",
	"auto_extra",
}

// autoModelRequestOptions reads the caller-facing auto-model parameters of
// the request. Everything is optional and forgiving: an unknown preference
// degrades to balanced, an invalid number to no bound.
func autoModelRequestOptions(body []byte) service.AutoModelRequestOptions {
	opts := service.AutoModelRequestOptions{Budget: int(gjson.GetBytes(body, "auto_budget").Int())}
	if opts.Budget <= 0 {
		opts.Budget = int(gjson.GetBytes(body, "auto_budget_tokens").Int())
	}
	opts.Preference = gjson.GetBytes(body, "auto_preference").String()
	if opts.Preference == "" {
		opts.Preference = gjson.GetBytes(body, "auto_prefer").String()
	}
	opts.MinPrice = gjson.GetBytes(body, "auto_price_min").Float()
	opts.MaxPrice = gjson.GetBytes(body, "auto_price_max").Float()
	opts.Instruction = gjson.GetBytes(body, "auto_instruction").String()
	if opts.Instruction == "" {
		opts.Instruction = gjson.GetBytes(body, "auto_extra").String()
	}
	opts.Instruction = strings.TrimSpace(opts.Instruction)
	if len(opts.Instruction) > 500 {
		opts.Instruction = opts.Instruction[:500]
	}
	return opts
}

// rewriteRequestModel writes the chosen model into the request body.
//
// A JSON request carries its model in the document. A speech to text request is
// a multipart form whose model field has to be rewritten as a form value, which
// is why the body is rebuilt with a fresh boundary: the body of the request is
// replaced as a whole, so the relay and the adapter see one consistent form.
func rewriteRequestModel(c *gin.Context, body []byte, chosen string) ([]byte, string, error) {
	if gjson.ValidBytes(body) {
		rewritten, err := sjson.SetBytes(body, "model", chosen)
		if err != nil {
			return nil, "", fmt.Errorf("failed to write the chosen model into the request")
		}
		for _, key := range autoRequestParamKeys {
			rewritten, err = sjson.DeleteBytes(rewritten, key)
			if err != nil {
				return nil, "", fmt.Errorf("failed to remove the auto-model parameters from the request")
			}
		}
		return rewritten, "", nil
	}

	mediaType, params, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		return nil, "", fmt.Errorf("the request of the auto model must be JSON or a multipart form")
	}
	boundary := params["boundary"]
	if boundary == "" {
		return nil, "", fmt.Errorf("the request of the auto model has no multipart boundary")
	}

	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	written := false
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", err
		}
		if part.FormName() == "model" && part.FileName() == "" {
			if _, err := io.Copy(io.Discard, part); err != nil {
				return nil, "", err
			}
			if err := writer.WriteField("model", chosen); err != nil {
				return nil, "", err
			}
			written = true
			continue
		}
		if strings.HasPrefix(part.FormName(), "auto_") {
			// The gateway consumes the auto-model parameters; do not
			// forward them upstream.
			if _, err := io.Copy(io.Discard, part); err != nil {
				return nil, "", err
			}
			continue
		}
		header := make(textproto.MIMEHeader)
		for key, values := range part.Header {
			for _, value := range values {
				header.Add(key, value)
			}
		}
		target, err := writer.CreatePart(header)
		if err != nil {
			return nil, "", err
		}
		if _, err := io.Copy(target, part); err != nil {
			return nil, "", err
		}
	}
	if !written {
		if err := writer.WriteField("model", chosen); err != nil {
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return buffer.Bytes(), writer.FormDataContentType(), nil
}

// autoModelRequestProfile extracts the text of the request and the kinds of
// content it carries, which is what the router needs to read.
func autoModelRequestProfile(body []byte, path string) (string, bool, bool) {
	hasAudio := strings.Contains(path, "/audio/transcriptions") ||
		strings.Contains(path, "/audio/translations") ||
		strings.Contains(path, "/audio/speech")

	hasImage := false
	texts := make([]string, 0, 8)
	if gjson.ValidBytes(body) {
		if parts := gjson.GetBytes(body, "messages"); parts.IsArray() {
			for _, message := range parts.Array() {
				if content := message.Get("content"); content.IsArray() {
					for _, part := range content.Array() {
						switch part.Get("type").String() {
						case "text", "input_text":
							texts = append(texts, part.Get("text").String())
						case "image_url", "input_image", "image":
							hasImage = true
						case "input_audio", "audio":
							hasAudio = true
						}
					}
					continue
				}
				if value := message.Get("content").String(); value != "" {
					texts = append(texts, value)
				}
			}
		}
		if prompt := gjson.GetBytes(body, "prompt"); prompt.Exists() {
			texts = append(texts, prompt.String())
		}
		if input := gjson.GetBytes(body, "input"); input.Type == gjson.String {
			texts = append(texts, input.String())
		}
	}
	return strings.Join(texts, "\n"), hasImage, hasAudio
}

// readReusableBody returns the body of the request from the storage that the
// relay reads later, so a rewrite is seen by every later step.
func readReusableBody(c *gin.Context) ([]byte, error) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, err
	}
	if _, err := storage.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return io.ReadAll(storage)
}

// replaceReusableBody stores the rewritten body as the body of the request.
func replaceReusableBody(c *gin.Context, body []byte) error {
	storage, err := common.CreateBodyStorage(body)
	if err != nil {
		return err
	}
	c.Set(common.KeyBodyStorage, storage)
	c.Request.Body = io.NopCloser(storage)
	c.Request.ContentLength = int64(len(body))
	c.Request.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
	return nil
}

// autoModelRequested reports whether the request asked for the virtual model.
func autoModelRequested(c *gin.Context) bool {
	_, ok := c.Get(autoModelDecisionKey)
	return ok
}

// AutoModelDecision returns the decision of the router of the current request.
func AutoModelDecision(c *gin.Context) (service.AutoModelSelection, bool) {
	value, ok := c.Get(autoModelDecisionKey)
	if !ok {
		return service.AutoModelSelection{}, false
	}
	selection, ok := value.(service.AutoModelSelection)
	return selection, ok
}

// tokenAllowsAutoModel reports whether a token that limits its models accepts
// the virtual model.
func tokenAllowsAutoModel(limits map[string]bool) bool {
	if limits == nil {
		return false
	}
	_, ok := limits[operation_setting.AutoModelName]
	return ok
}

// autoModelAbort writes the error of a request that could not be routed.
func autoModelAbort(c *gin.Context, err error) {
	abortWithOpenAiMessage(c, http.StatusBadRequest, err.Error())
}
