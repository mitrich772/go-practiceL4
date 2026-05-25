package protocol

// GrepFlags повторяет подмножество флагов GNU grep, которое поддерживает
// распределённая реализация.
type GrepFlags struct {
	Pattern      string `json:"pattern"`
	IgnoreCase   bool   `json:"ignore_case"`    // -i
	InvertMatch  bool   `json:"invert_match"`   // -v
	FixedString  bool   `json:"fixed_string"`   // -F
	PrintLineNum bool   `json:"print_line_num"` // -n
	CountOnly    bool   `json:"count_only"`     // -c
	ListFiles    bool   `json:"list_files"`     // -l
}

// ProcessRequest — задание серверу обработать один чанк входных данных.
//
// ChunkID и StartLine позволяют клиенту корректно собрать итоговый результат
// в исходном порядке и подставить настоящие номера строк, даже если ответы
// серверов приходят в произвольном порядке.
type ProcessRequest struct {
	ChunkID   int       `json:"chunk_id"`
	StartLine int       `json:"start_line"` // 1-based номер первой строки чанка
	FileName  string    `json:"file_name,omitempty"`
	Flags     GrepFlags `json:"flags"`
	Data      string    `json:"data"`
}

// Match — одна совпавшая строка с абсолютным номером.
type Match struct {
	LineNum int    `json:"line_num"`
	Text    string `json:"text"`
}

// ProcessResponse — ответ сервера на ProcessRequest.
//
// Сервер всегда возвращает ChunkID, чтобы клиент мог сопоставить ответ с
// заданием при асинхронном сборе через канал.
type ProcessResponse struct {
	ChunkID  int     `json:"chunk_id"`
	Matches  []Match `json:"matches,omitempty"`
	Count    int     `json:"count,omitempty"`
	HasMatch bool    `json:"has_match,omitempty"`
	Error    string  `json:"error,omitempty"`
}

// HealthResponse возвращается на GET /healthz и используется в e2e-скриптах,
// чтобы дождаться готовности сервера.
type HealthResponse struct {
	Status string `json:"status"`
}
