package file

import (
	"fmt"
	"io"
	"os"
)

func CopyFile(src, dst string) (int64, error) {
	// 打开源文件
	sourceFile, err := os.Open(src)
	if err != nil {
		return 0, fmt.Errorf("failed to open source file: %v", err)
	}
	defer sourceFile.Close()

	// 创建目标文件
	destFile, err := os.Create(dst)
	if err != nil {
		return 0, fmt.Errorf("failed to create destination file: %v", err)
	}
	defer destFile.Close()

	// 复制文件内容
	n, err := io.Copy(destFile, sourceFile)
	if err != nil {
		return 0, fmt.Errorf("failed to copy file: %v", err)
	}

	return n, nil
}
