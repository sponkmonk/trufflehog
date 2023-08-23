package s3

import (
	"encoding/base64"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/kylelemons/godebug/pretty"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/trufflesecurity/trufflehog/v3/pkg/common"
	"github.com/trufflesecurity/trufflehog/v3/pkg/context"
	"github.com/trufflesecurity/trufflehog/v3/pkg/pb/credentialspb"
	"github.com/trufflesecurity/trufflehog/v3/pkg/pb/sourcespb"
	"github.com/trufflesecurity/trufflehog/v3/pkg/sources"
)

func TestSource_Chunks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	secret, err := common.GetTestSecret(ctx)
	if err != nil {
		t.Fatal(fmt.Errorf("failed to access secret: %v", err))
	}

	s3key := secret.MustGetField("AWS_S3_KEY")
	s3secret := secret.MustGetField("AWS_S3_SECRET")

	type init struct {
		name       string
		verify     bool
		connection *sourcespb.S3
	}
	tests := []struct {
		name          string
		init          init
		wantErr       bool
		wantChunkData string
	}{
		{
			name: "gets chunks",
			init: init{
				connection: &sourcespb.S3{
					Credential: &sourcespb.S3_AccessKey{
						AccessKey: &credentialspb.KeySecret{
							Key:    s3key,
							Secret: s3secret,
						},
					},
					Buckets: []string{"truffletestbucket-s3-tests"},
				},
			},
			wantErr:       false,
			wantChunkData: `W2RlZmF1bHRdCmF3c19hY2Nlc3Nfa2V5X2lkID0gQUtJQTM1T0hYMkRTT1pHNjQ3TkgKYXdzX3NlY3JldF9hY2Nlc3Nfa2V5ID0gUXk5OVMrWkIvQ1dsRk50eFBBaWQ3Z0d6dnNyWGhCQjd1ckFDQUxwWgpvdXRwdXQgPSBqc29uCnJlZ2lvbiA9IHVzLWVhc3QtMg==`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
			var cancelOnce sync.Once
			defer cancelOnce.Do(cancel)

			s := Source{}
			conn, err := anypb.New(tt.init.connection)
			if err != nil {
				t.Fatal(err)
			}

			err = s.Init(ctx, tt.init.name, 0, 0, tt.init.verify, conn, 8)
			if (err != nil) != tt.wantErr {
				t.Errorf("Source.Init() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			chunksCh := make(chan *sources.Chunk)
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				err = s.Chunks(ctx, chunksCh)
				if (err != nil) != tt.wantErr {
					t.Errorf("Source.Chunks() error = %v, wantErr %v", err, tt.wantErr)
					os.Exit(1)
				}
			}()
			gotChunk := <-chunksCh
			wantData, _ := base64.StdEncoding.DecodeString(tt.wantChunkData)

			if diff := pretty.Compare(gotChunk.Data, wantData); diff != "" {
				t.Errorf("%s: Source.Chunks() diff: (-got +want)\n%s", tt.name, diff)
			}
			wg.Wait()
			assert.Equal(t, "", s.GetProgress().EncodedResumeInfo)
			assert.Equal(t, int64(100), s.GetProgress().PercentComplete)
		})
	}
}
