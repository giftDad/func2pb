# usage
```bash
go install github.com/giftDad/func2pb@latest
func2pb -file test/test.go -out test/test.proto
```

# example
```go
package test

import "context"

// 用户
type User struct {
        ID   int64  // ID
        Name string // 名字
        Age  int64  // 年龄
        Ext  Ext    // 信息
}

type Ext struct {
        A int // A
        B int // B
        C int // C
}

// List 列表
func List(ctx context.Context, limit, offset int32) ([]User, error) {
        return []User{}, nil
}

// Add 新增
func Add(ctx context.Context, u User) error {
        return nil
}
```

```protobuf
syntax = "proto3";

// TODO fill it
package xxx;

service Test {
	// List 列表
	rpc List(ListReq) returns (ListResp);
	// Add 新增
	rpc Add(AddReq) returns (AddResp);
	
}


message ListReq {
	int32 limit = 1;
	int32 offset = 2;
	
}

message ListResp {
	
}

message AddReq {
	User u = 1;
	
}

message AddResp {
	
}



// 用户
message User {
	// ID
	int64 ID = 1;
	// 名字
	string Name = 2;
	// 年龄
	int64 Age = 3;
	// 信息
	Ext Ext = 4;
	
}

// 
message Ext {
	// A
	int A = 1;
	// B
	int B = 2;
	// C
	int C = 3;
	
}
```
