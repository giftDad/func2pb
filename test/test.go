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
