package config

import "testing"

func TestIsExcluded(t *testing.T) {
	tests := []struct {
		name    string
		exclude []string
		path    string
		want    bool
	}{
		// 名称形式：按完整路径段匹配，不做子串误伤
		{"名称精确命中", []string{"logs"}, `C:\Users\me\AppData\Roaming\App\logs`, true},
		{"子串不误伤", []string{"logs"}, `C:\Users\me\Catalogs`, false},
		{"子串不误伤2", []string{"temp"}, `C:\Users\me\Temporary`, false},
		{"名称大小写不敏感", []string{"User Data"}, `C:\Users\me\AppData\Local\Doubao\user data\Default\Cache`, true},
		{"中间段命中", []string{"Trae CN"}, `C:\Users\me\AppData\Roaming\Trae CN\Cache`, true},
		// 多段名称：按段后缀匹配
		{"多段名称命中", []string{"AppData/Local/Temp"}, `C:\Users\me\AppData\Local\Temp`, true},
		{"多段名称不完整不命中", []string{"Local/Temp"}, `C:\Users\me\AppData\Roaming\Temp`, false},
		// 绝对路径形式：祖先包含
		{"绝对路径自身命中", []string{`C:\Users\me\.ollama`}, `C:\Users\me\.ollama`, true},
		{"绝对路径子孙命中", []string{`C:\Users\me\.ollama`}, `C:\Users\me\.ollama\models`, true},
		{"绝对路径大小写不敏感", []string{`C:\USERS\ME\.ollama`}, `c:\users\me\.ollama\models`, true},
		{"绝对路径前缀字符串但不越界", []string{`C:\Users\me\.ol`}, `C:\Users\me\.ollama\models`, false},
		// 其他
		{"无排除项", nil, `C:\Users\me\Cache`, false},
		{"排除项为空串", []string{"", "  "}, `C:\Users\me\Cache`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{ExcludeDirs: tt.exclude}
			if got := cfg.IsExcluded(tt.path); got != tt.want {
				t.Errorf("IsExcluded(%q) with exclude %v = %v, want %v", tt.path, tt.exclude, got, tt.want)
			}
		})
	}
}
