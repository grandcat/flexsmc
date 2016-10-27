package smc

import "testing"

// func Test_connect(t *testing.T) {
// 	type args struct {
// 		socket string
// 		id     int32
// 	}
// 	tests := []struct {
// 		name string
// 		args args
// 	}{
// 		{
// 			name: "p1",
// 			args: args{
// 				socket: "unix:///tmp/grpc1.sock",
// 				id:     1,
// 			},
// 		},
// 		{
// 			name: "p2",
// 			args: args{
// 				socket: "unix:///tmp/grpc2.sock",
// 				id:     2,
// 			},
// 		},
// 		{
// 			name: "p3",
// 			args: args{
// 				socket: "unix:///tmp/grpc3.sock",
// 				id:     3,
// 			},
// 		},
// 	}
// 	for _, tt := range tests {
// 		connect(tt.args.socket, tt.args.id)
// 	}
// }

func Test_ParallelConnect(t *testing.T) {
	type args struct {
		socket string
		id     int32
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "p1",
			args: args{
				socket: "unix:///tmp/grpc1.sock",
				id:     1,
			},
		},
		{
			name: "p2",
			args: args{
				socket: "unix:///tmp/grpc2.sock",
				id:     2,
			},
		},
		{
			name: "p3",
			args: args{
				socket: "unix:///tmp/grpc3.sock",
				id:     3,
			},
		},
	}

	for _, tc := range tests {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			connect(tc.args.socket, tc.args.id)
		})
	}
}
