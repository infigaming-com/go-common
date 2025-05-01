package request

type RequestRecorder func(requestSigningData *RequestRecordData)

type RequestRecordData struct {
	Method         string
	Url            string
	QueryParams    string
	RequestHeaders string
	RequestBody    string
	HttpStatusCode int
	ResponseBody   string
	Error          string
	Duration       int64
}
