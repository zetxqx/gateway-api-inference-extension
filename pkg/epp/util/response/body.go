/*
Copyright 2025 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package response

import (
	errutil "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/util/error"
)

// ExtractContentFromResponseBody parses a model's JSON response and extracts the
// content from the first choice. For a chat completion, this is the assistant's message.
func ExtractContentFromResponseBody(body map[string]any) (string, error) {
	choices, ok := body["choices"]
	if !ok {
		return "", errutil.Error{Code: errutil.BadRequest, Msg: "'choices' field not found in response body"}
	}
	choiceList, ok := choices.([]any)
	if !ok {
		return "", errutil.Error{Code: errutil.BadRequest, Msg: "'choices' field is not a list"}
	}
	if len(choiceList) == 0 {
		return "", errutil.Error{Code: errutil.BadRequest, Msg: "'choices' list is empty"}
	}

	firstChoice, ok := choiceList[0].(map[string]any)
	if !ok {
		return "", errutil.Error{Code: errutil.BadRequest, Msg: "first item in 'choices' is not an object"}
	}

	message, ok := firstChoice["message"]
	if !ok {
		return "", errutil.Error{Code: errutil.BadRequest, Msg: "'message' field not found in the first choice"}
	}
	messageMap, ok := message.(map[string]any)
	if !ok {
		return "", errutil.Error{Code: errutil.BadRequest, Msg: "'message' field is not an object"}
	}

	content, ok := messageMap["content"]
	if !ok {
		return "", errutil.Error{Code: errutil.BadRequest, Msg: "'content' field not found in the message object"}
	}
	contentStr, ok := content.(string)
	if !ok {
		return "", errutil.Error{Code: errutil.BadRequest, Msg: "'content' field is not a string"}
	}

	return contentStr, nil
}

// Example

// curl http://${IP}/v1/chat/completions \
//   -H "Content-Type: application/json" \
//   -d '{
//     "model": "google/gemma-2-2b-it",
//     "messages": [
//       {
//         "role": "user",
//         "content": "Hello!"
//       }
//     ]
//   }'
// {"choices":[{"finish_reason":"stop","index":0,"logprobs":null,"message":{"content":"Hello! 👋  \n\nHow can I help you today? 😊 \n","reasoning_content":null,"role":"assistant","tool_calls":[]},"stop_reason":107}],"created":1755275445,"id":"chatcmpl-4ebc2a35fe9d4185b58f44a94de5476f","model":"google/gemma-2-2b-it","object":"chat.completion","prompt_logprobs":null,"usage":{"completion_tokens":16,"prompt_tokens":11,"prompt_tokens_details":null,"total_tokens":27}}%
