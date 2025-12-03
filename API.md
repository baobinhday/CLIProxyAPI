
# 🌐 Các Endpoint API được Cung cấp (Exposed API Endpoints)

Dưới đây là danh sách các endpoint API chính được cung cấp bởi project, được nhóm theo định dạng API tương thích.

## 1\. 🤖 Endpoint Tương thích với OpenAI (`/v1`)

Các endpoint này yêu cầu payload ở định dạng OpenAI và được bảo vệ bằng xác thực (`AuthMiddleware`).

| Phương thức | Đường dẫn (Path) | Chức năng (Handler) | Mô tả |
| :--- | :--- | :--- | :--- |
| `GET` | `/v1/models` | `unifiedModelsHandler` | Liệt kê tất cả các mô hình AI có sẵn (Gemini, Claude, v.v.) theo định dạng OpenAI. |
| `POST` | `/v1/chat/completions` | `openaiHandlers.ChatCompletions` | Tạo nội dung chat. **Yêu cầu payload định dạng OpenAI Chat Completions.** |
| `POST` | `/v1/completions` | `openaiHandlers.Completions` | Tạo nội dung (API cũ của OpenAI). **Chuyển đổi nội bộ request sang định dạng Chat Completions.** |

## 2\. 💎 Endpoint Tương thích với Gemini (`/v1beta`)

Các endpoint này yêu cầu payload ở định dạng Gemini và được bảo vệ bằng xác thực (`AuthMiddleware`).

| Phương thức | Đường dẫn (Path) | Chức năng (Handler) | Mô tả |
| :--- | :--- | :--- | :--- |
| `GET` | `/v1beta/models` | `geminiHandlers.GeminiModels` | Liệt kê các mô hình Gemini có sẵn. |
| `POST` | `/v1beta/models/:action` | `geminiHandlers.GeminiHandler` | Thực hiện các hành động trên mô hình (ví dụ: `gemini-pro:generateContent` hoặc `gemini-pro:streamGenerateContent`). |
| `GET` | `/v1beta/models/:action` | `geminiHandlers.GeminiGetHandler` | Lấy thông tin chi tiết về một mô hình Gemini cụ thể. |

## 3\. 🧩 Endpoint Aliases của Nhà cung cấp (Provider Aliases - Amp Module)

Các tuyến này được đăng ký thông qua `AmpModule` và cho phép gọi các API của nhà cung cấp khác nhau thông qua định dạng đường dẫn chung `/api/provider/:provider/...`.

| Phương thức | Đường dẫn (Path) | Handler (Fallback) | Định dạng Payload |
| :--- | :--- | :--- | :--- |
| `POST` | `/api/provider/:provider/chat/completions` | `openaiHandlers.ChatCompletions` | OpenAI Chat Completions |
| `POST` | `/api/provider/:provider/v1/chat/completions` | `openaiHandlers.ChatCompletions` | OpenAI Chat Completions |
| `POST` | `/api/provider/:provider/v1/messages` | `claudeCodeHandlers.ClaudeMessages` | Claude/Anthropic Messages |
| `POST` | `/api/provider/google/v1beta/models/:action` | `geminiHandlers.GeminiHandler` | Gemini API |
| `GET` | `/api/provider/:provider/models` | `ampModelsHandler` | N/A |

**Lưu ý:** Các endpoint của Amp Module cũng cung cấp tính năng **fallback** (chuyển tiếp yêu cầu đến upstream Amp nếu không thể xử lý cục bộ) và được bảo vệ bằng xác thực.

## 4\. 🔒 Endpoint Quản lý/Nội bộ (Hạn chế Localhost)

Các tuyến này **chủ yếu được giới hạn ở localhost** và không dành cho việc sử dụng API thông thường.

| Phương thức | Đường dẫn (Path) | Chức năng (Handler) | Mô tả |
| :--- | :--- | :--- | :--- |
| `ANY` | `/api/internal*`, `/api/user*`, `/api/auth*`, `/api/meta*` | `proxyHandler` | Proxy các yêu cầu quản lý/xác thực nội bộ của Amp. |
| `POST` | `/v1internal:generateContent` | `geminiCLIHandlers...` | Nội bộ cho Gemini CLI (chỉ localhost). |
| `POST` | `/v1internal:streamGenerateContent` | `geminiCLIHandlers...` | Nội bộ cho Gemini CLI (chỉ localhost). |