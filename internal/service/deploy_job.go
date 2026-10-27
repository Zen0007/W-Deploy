package service

// DeployJob adalah pekerjaan clone -> build -> deploy yang diproses worker.
type DeployJob struct {
	AppID           uint
	DeploymentID    uint
	ZipPath         string // diisi jika source = zip (sudah tersimpan di workspace)
	DBEngine        string // opsional: provision database terisolasi sebelum build
	ProjectTypeHint string // snapshot hint dari FE, dibawa ke worker
}

// DeployEnqueuer diimplementasikan oleh worker.Queue (menghindari import siklik).
type DeployEnqueuer interface {
	Enqueue(job DeployJob) error
}

// LogSink menerima event real-time untuk satu app (diimplementasikan websocket.Hub).
type LogSink interface {
	Publish(room string, event any)
	ResetHistory(room string)
}
