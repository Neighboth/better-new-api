package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmailTemplatesCleanDesignAndAllLocales(t *testing.T) {
	all := GetAllDefaultEmailTemplates()
	expectedLangs := []string{"tr", "en", "zh_CN", "zh_TW", "fr", "ru", "ja", "vi"}
	expectedTypes := []string{
		"verification",
		"password_reset",
		"system_error",
		"quota_warning",
		"ticket_created",
		"ticket_replied",
		"channel_disabled",
		"video_expiry",
		"fine_tuning_expiry",
	}

	for _, lang := range expectedLangs {
		tplMap, ok := all[lang]
		require.True(t, ok, "missing locale %s in defaultEmailTemplates", lang)
		for _, typ := range expectedTypes {
			tpl, exists := tplMap[typ]
			require.True(t, exists, "missing template %s for locale %s", typ, lang)
			assert.NotEmpty(t, tpl.Subject, "empty subject for %s/%s", lang, typ)
			assert.NotEmpty(t, tpl.Body, "empty body for %s/%s", lang, typ)

			// Verify clean modern reference palette & structure
			assert.Contains(t, tpl.Body, "#9EB7EE", "missing periwinkle canvas in %s/%s", lang, typ)
			assert.Contains(t, tpl.Body, "#4D62E5", "missing solid indigo header in %s/%s", lang, typ)
			assert.Contains(t, tpl.Body, "{{system_name}}", "missing system_name header in %s/%s", lang, typ)

			// Ensure anti-slop invariants: no gradients, no SVG icons, no images/avatars, no social links
			assert.NotContains(t, tpl.Body, "linear-gradient", "no gradients allowed in %s/%s", lang, typ)
			assert.NotContains(t, tpl.Body, "<svg", "no SVG icons allowed in %s/%s", lang, typ)
			assert.NotContains(t, tpl.Body, "<img", "no avatar/social images allowed in %s/%s", lang, typ)
			assert.NotContains(t, tpl.Body, "linkedin.com", "no social icons in %s/%s", lang, typ)
		}
	}
}

func TestRenderEmailFineTuningExpirySubstitutesVariables(t *testing.T) {
	subj, body := RenderEmail("fine_tuning_expiry", "tr", map[string]string{
		"username":     "salih",
		"model":        "ft:Qwen/Qwen2.5-7B:demo",
		"job_id":       "ftjob-abc123",
		"days_left":    "3",
		"expires_at":   "2026-12-01",
		"download_url": "https://api.example.com/custom-models",
	})
	assert.Contains(t, subj, "3")
	assert.Contains(t, body, "ft:Qwen/Qwen2.5-7B:demo")
	assert.Contains(t, body, "salih")
	assert.Contains(t, body, "ftjob-abc123")
	assert.Contains(t, body, "2026-12-01")
	assert.Contains(t, body, "https://api.example.com/custom-models")
	assert.NotContains(t, body, "{{username}}")
	assert.NotContains(t, body, "{{download_url}}")
}
