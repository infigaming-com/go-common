package util

import "strings"

// makeAllowedOriginValidator 构建动态跨域验证函数。
// 特性：
// 1. 支持 "*" 全放行
// 2. 支持精确匹配
// 3. 支持 "https://*.example.com" 模糊匹配
// 4. 支持日志输出（可选）
func MakeAllowedOriginValidator(allowedOrigins []string) func(origin string) bool {
	// 是否允许所有
	for _, o := range allowedOrigins {
		if o == "*" {
			//log.Println("[CORS] Allow all origins ('*' configured)")
			return func(origin string) bool {
				return true
			}
		}
	}

	//log.Printf("[CORS] Allowed origins: %v\n", allowedOrigins)

	return func(origin string) bool {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			return false
		}

		for _, allowed := range allowedOrigins {
			allowed = strings.TrimSpace(allowed)

			// 精确匹配
			if allowed == origin {
				//log.Printf("[CORS] ✅ Allowed: %s (exact match)\n", origin)
				return true
			}

			// 自动去掉 https:// 或 http:// 再比
			originHost := strings.TrimPrefix(strings.TrimPrefix(origin, "https://"), "http://")
			if originHost == allowed {
				return true
			}

			// 模糊匹配（支持 *.example.com）
			if strings.Contains(allowed, "*") {
				prefix := strings.Split(allowed, "*")[0]
				suffix := strings.Split(allowed, "*")[1]
				if strings.HasPrefix(origin, prefix) && strings.HasSuffix(origin, suffix) {
					//log.Printf("[CORS] ✅ Allowed: %s (wildcard match: %s)\n", origin, allowed)
					return true
				}
			}
		}

		//log.Printf("[CORS] ❌ Denied: %s\n", origin)
		return false
	}
}
