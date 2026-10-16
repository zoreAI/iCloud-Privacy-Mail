# Next.js Web 工作台

前端位于 `web/`，独立使用 Next.js App Router、React、TypeScript。Go 服务保留业务逻辑和认证；前端同源代理 `/api/*`，不向浏览器暴露配置中的凭证。

开发：先在仓库根目录运行 `go run ./cmd/panel`，再在 `web/` 执行 `npm install` 和 `npm run dev`，访问 `http://127.0.0.1:3000`。

构建：`npm run build`；运行：`npm start`。后端地址通过 `GO_BACKEND_URL` 配置（默认 `http://127.0.0.1:8787`），在构建时设置。生产环境把反向代理指向 Next.js 端口，并让 Go 仅监听内部地址。

此版本仍在迁移中：邮箱管理、登录、任务及导出已开始接入；完整功能回归、浏览器交互验证和部署说明需要完成后再替代原面板。旧 Go HTML 页面当前保留；迁移本身不会修改现有数据或自动执行删除。
