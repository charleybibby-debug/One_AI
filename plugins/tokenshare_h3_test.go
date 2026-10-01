package plugins_test

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	builtinplugins "github.com/QuantumNous/new-api/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenShareH3BuildsAsyncVideoRequest(t *testing.T) {
	plugin := loadTokenShareH3Plugin(t)
	value, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", map[string]any{
		"requestBody": map[string]any{"model": "tokenshare-minimax-h3", "prompt": "a sunrise", "duration": 5, "size": "16:9"},
		"model":       "tokenshare-minimax-h3", "upstreamModel": "tokenshare-minimax-h3", "baseUrl": "https://api.tokenshare.net", "apiKey": "test-key", "publicTaskId": "task-public",
	})
	require.NoError(t, err)
	var descriptor map[string]any
	encoded, err := common.Marshal(value)
	require.NoError(t, err)
	require.NoError(t, common.Unmarshal(encoded, &descriptor))
	assert.Equal(t, "https://api.tokenshare.net/v1/videos", descriptor["url"])
	assert.Equal(t, "POST", descriptor["method"])
	assert.Equal(t, "text_to_video", descriptor["action"])
	body, err := common.Marshal(descriptor["body"])
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"minimax-h3","task":"t2va","prompt":"a sunrise","target":{"short_edge":768,"aspect_ratio":"16:9","duration_seconds":5}}`, string(body))
	headers := descriptor["headers"].(map[string]any)
	assert.Equal(t, "new-api-task-public", headers["Idempotency-Key"])
}

func TestTokenShareH3MapsResponsesImagesToKeyframes(t *testing.T) {
	plugin := loadTokenShareH3Plugin(t)
	value, err := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_responses", "decodeRequest"}, map[string]any{
		"body": map[string]any{"kind": "json", "value": map[string]any{
			"model": "tokenshare-minimax-h3",
			"input": []any{map[string]any{"role": "user", "content": []any{
				map[string]any{"type": "input_text", "text": "animate this"},
				map[string]any{"type": "input_image", "image_url": "https://cdn.example/a.png"},
			}}},
		}},
		"model": "tokenshare-minimax-h3",
	})
	require.NoError(t, err)
	var intent map[string]any
	encoded, err := common.Marshal(value)
	require.NoError(t, err)
	require.NoError(t, common.Unmarshal(encoded, &intent))
	assert.Equal(t, "image_to_video", intent["action"])
	requestBody := intent["requestBody"].(map[string]any)
	assert.Equal(t, "tokenshare-minimax-h3", requestBody["model"])
	conditions := requestBody["conditions"].([]any)
	assert.Equal(t, "keyframe", conditions[0].(map[string]any)["role"])
	assert.Equal(t, float64(0), conditions[0].(map[string]any)["frame_index"])
}

func TestTokenShareH3ParsesLifecycleAndBoundsUsage(t *testing.T) {
	plugin := loadTokenShareH3Plugin(t)
	for _, testCase := range []struct {
		body string
		want string
	}{
		{`{"id":"v1","status":"queued"}`, "QUEUED"},
		{`{"id":"v1","status":"running"}`, "IN_PROGRESS"},
		{`{"id":"v1","status":"processing"}`, "IN_PROGRESS"},
		{`{"id":"v1","status":"completed"}`, "SUCCESS"},
		{`{"id":"v1","status":"failed","error":{"message":"blocked"}}`, "FAILURE"},
	} {
		t.Run(testCase.want, func(t *testing.T) {
			var body any
			require.NoError(t, common.UnmarshalJsonStr(testCase.body, &body))
			value, err := plugin.Engine.Call(t.Context(), "parseTaskResult", map[string]any{}, body)
			require.NoError(t, err)
			var result map[string]any
			encoded, marshalErr := common.Marshal(value)
			require.NoError(t, marshalErr)
			require.NoError(t, common.Unmarshal(encoded, &result))
			assert.Equal(t, testCase.want, result["status"])
		})
	}
	var body any
	require.NoError(t, common.UnmarshalJsonStr(`{"id":"v1","status":"completed","target":{"duration_seconds":5},"usage":{"output_seconds":16,"input_image_count":3,"input_seconds":20}}`, &body))
	value, err := plugin.Engine.Call(t.Context(), "extractUsageOnComplete", nil, nil, body)
	require.NoError(t, err)
	var facts map[string]any
	encoded, err := common.Marshal(value)
	require.NoError(t, err)
	require.NoError(t, common.Unmarshal(encoded, &facts))
	assert.Nil(t, facts)
}

func loadTokenShareH3Plugin(t *testing.T) *jsplugin.LoadedPlugin {
	t.Helper()
	source, err := builtinplugins.Source("tokenshare-h3")
	require.NoError(t, err)
	plugin, err := jsplugin.NewRegistry().RegisterFactory(source, jsplugin.Options{Key: "tokenshare-h3"})
	require.NoError(t, err)
	return plugin
}
