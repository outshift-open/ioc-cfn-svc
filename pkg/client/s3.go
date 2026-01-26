package client

type S3 interface {
	UploadFile(bucket, filepath string, data []byte) (string, error)
}

// ensure at build time that this mock type fulfills the interface
var _ S3 = (*MockS3)(nil)

type MockS3 struct{}

func NewMockS3() *MockS3 {
	log.Warn("creating mock [s3] service")
	return &MockS3{}
}

func (m *MockS3) UploadFile(bucket, filepath string, data []byte) (string, error) {
	return "", nil
}
