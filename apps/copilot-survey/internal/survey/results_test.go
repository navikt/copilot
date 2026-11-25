package survey

import (
	"encoding/json"
)

func row(answers, versions string) StoredAnswer {
	var a StoredAnswer
	if json.Unmarshal([]byte(`{"answers":`+answers+`,"question_versions":`+versions+`}`), &a) != nil {
		panic(answers)
	}
	return a
}
