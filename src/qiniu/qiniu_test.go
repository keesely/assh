// qiniu_test.go kee > 2019/11/29

package qiniu

import (
	"fmt"
	"log"
	"testing"
)

var e error

func TestQiniu(t *testing.T) {
	qNiu := New(
		"40pTySHejJkY9_7qj5mYbfZAQEmJYmiQDx79ycA8",
		"lBbl_xL8LBBICqLVH9CFKa4xLnLpto_PEhEy5ksd",
		"kocs",
	)

	e = qNiu.Upload("./test.txt", "test/test.txt")
	if e != nil {
		log.Fatal("Upload: ", e)
	}
	fmt.Println("上传完成")

	e = qNiu.Download("test/test.txt", "test2.txt")
	if nil != e {
		log.Fatal("Download: ", e)
	}
	fmt.Println("下载完成")
}
