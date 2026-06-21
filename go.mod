module aurora-dispatchers-llm

go 1.26

require (
	aurora-dispatchers v0.0.0
	capcompute v0.0.0
	github.com/openai/openai-go/v3 v3.41.0
)

require (
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
)

replace aurora-dispatchers => ../aurora-dispatchers

replace capcompute => ../capcompute
