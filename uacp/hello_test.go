// Copyright 2018-2019 opcua authors. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be
// found in the LICENSE file.

package uacp

import (
	"testing"

	"github.com/gopcua/opcua/ua"
)

func TestHello(t *testing.T) {
	cases := []ua.CodecTestCase{
		{
			Struct: NewHello(
				0,                                        // Version
				65280,                                    // ReceiveBufSize
				65535,                                    // SendBufSize
				4000,                                     // MaxMessageSize
				"opc.tcp://wow.its.easy:11111/UA/Server", // EndPointURL
			),
			Bytes: []byte{ // Hello message
				// Version: 0
				0x00, 0x00, 0x00, 0x00,
				// ReceiveBufSize: 65280
				0x00, 0xff, 0x00, 0x00,
				// SendBufSize: 65535
				0xff, 0xff, 0x00, 0x00,
				// MaxMessageSize: 4000
				0xa0, 0x0f, 0x00, 0x00,
				// MaxChunkCount: 0
				0x00, 0x00, 0x00, 0x00,
				// EndPointURL
				0x26, 0x00, 0x00, 0x00, 0x6f, 0x70, 0x63, 0x2e,
				0x74, 0x63, 0x70, 0x3a, 0x2f, 0x2f, 0x77, 0x6f,
				0x77, 0x2e, 0x69, 0x74, 0x73, 0x2e, 0x65, 0x61,
				0x73, 0x79, 0x3a, 0x31, 0x31, 0x31, 0x31, 0x31,
				0x2f, 0x55, 0x41, 0x2f, 0x53, 0x65, 0x72, 0x76,
				0x65, 0x72,
			},
		},
	}
	ua.RunCodecTest(t, cases)
}
