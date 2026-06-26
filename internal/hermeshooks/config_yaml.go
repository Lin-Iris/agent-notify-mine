package hermeshooks

// HermesConfig 描述 ~/.hermes/config.yaml 中与本插件相关的字段。
type HermesConfig struct {
	Hooks            *HermesHooksConfig `yaml:"hooks"`
	HooksAutoAccept  bool               `yaml:"hooks_auto_accept"`
}

// HermesHooksConfig 对应 hooks: 块。
type HermesHooksConfig struct {
	PostLLMCall         []HermesHookEntry `yaml:"post_llm_call,omitempty"`
	PreApprovalRequest  []HermesHookEntry `yaml:"pre_approval_request,omitempty"`
}

// HermesHookEntry 对应 shell hook 条目。
type HermesHookEntry struct {
	Command string `yaml:"command"`
	Timeout int    `yaml:"timeout,omitempty"`
}
