package finetunescript

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateFineTuneScriptContainsRealTrainingPipeline(t *testing.T) {
	script := Generate(Config{
		JobID:          "ftjob-real-01",
		FineTunedModel: "ft:Qwen/Qwen2.5-0.5B:demo",
		BaseModel:      "Qwen/Qwen2.5-0.5B",
		ModelSource:    "huggingface",
		DatasetURL:     "https://api.example.com/api/deployments/fine_tuning/ftjob-real-01/dataset",
		DatasetName:    "train.jsonl",
		TaskType:       "text",
		Method:         "lora",
		Epochs:         3,
		Accelerator:    "T4",
		CallbackURL:    "https://api.example.com/api/deployments/fine_tuning/ftjob-real-01/callback",
		ArtifactURL:    "https://api.example.com/api/deployments/fine_tuning/ftjob-real-01/artifact",
		CallbackToken:  "secret-cb-token",
	})

	require.NotEmpty(t, script)
	assert.Contains(t, script, `JOB_ID = "ftjob-real-01"`)
	assert.Contains(t, script, `BASE_MODEL = "Qwen/Qwen2.5-0.5B"`)
	assert.Contains(t, script, "AutoModelForCausalLM")
	assert.Contains(t, script, "LoraConfig")
	assert.Contains(t, script, "get_peft_model")
	assert.Contains(t, script, "loss.backward()")
	assert.Contains(t, script, "optimizer.step()")
	assert.Contains(t, script, "save_pretrained")
	assert.Contains(t, script, "safe_serialization=True")
	assert.Contains(t, script, "training_details.json")
	assert.Contains(t, script, "FT_LOCAL_DATASET")
	assert.Contains(t, script, "FT_LOCAL_BASE_MODEL")
	assert.Contains(t, script, "FT_LOCAL_RESUME_CHECKPOINT")
	assert.Contains(t, script, "split_hf_repo_and_file")
	assert.Contains(t, script, "save_and_upload_checkpoint")
	assert.Contains(t, script, "restore_resume_checkpoint")
	assert.Contains(t, script, "checkpoint_latest.zip")
	assert.Contains(t, script, "train_image_diffusion")
}

func TestGenerateEscapesQuotesAndDefaults(t *testing.T) {
	script := Generate(Config{
		JobID:     `ftjob-"quoted"`,
		BaseModel: `C:\models\my"model.safetensors`,
		Epochs:    0,
	})
	assert.Contains(t, script, `JOB_ID = "ftjob-\"quoted\""`)
	assert.Contains(t, script, `BASE_MODEL = "C:\\models\\my\"model.safetensors"`)
	assert.Contains(t, script, `EPOCHS = 3`)
	assert.Contains(t, script, `TASK_TYPE = "text"`)
	assert.Contains(t, script, `METHOD = "lora"`)
}
