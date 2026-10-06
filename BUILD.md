# LSmusic 构建指南（v1.1.1）

## 目录结构
- `backend/`    Go 后端（含 go:embed 前端产物 dist/）
- `frontend/`   Vue 3 前端源码（Vite 构建）
- `fpk-package/` 飞牛 FPK 打包目录（cmd 生命周期脚本 / config 权限声明 / manifest / 图标）
- `preset_sources/` 预置音源脚本

## 构建步骤（Linux x86_64）

### 1. 构建前端
```bash
cd frontend
npm install          # 首次
npm run build        # 产物输出到 ../backend/dist
```

### 2. 构建后端二进制（嵌入前端产物）
```bash
cd backend
GOFLAGS=-mod=mod CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -ldflags="-s -w" -o ../fpk-package/app/fn-lx-player .
```

### 3. 打包 FPK（手工打包，勿用 fnpack——其 app.tgz 存在 config 重复缺陷）
```bash
cd fpk-package/app
tar -czf ../app.tgz *            # 内容无 app/ 前缀
cd ..
NEWSUM=$(md5sum app.tgz | cut -d' ' -f1)
sed -i "s/^checksum.*/checksum = ${NEWSUM}/" manifest
tar -czf ../LSmusic_v1.1.1.fpk app.tgz cmd config ICON_256.PNG ICON.PNG manifest
```

### 4. 在飞牛 NAS 应用中心「手动安装」上传生成的 .fpk

## 关键约束
- `cmd/` 脚本必须保持纯 POSIX（dash 兼容）；`cmd/main` 与业务回调钩子需 755
- 版本号需同步修改：`fpk-package/manifest`、`backend/pkg/api/server.go`（/api/app/version）
- 授权目录通过飞牛官方机制读取：`TRIM_DATA_ACCESSIBLE_PATHS` 环境变量
  与 `$TRIM_PKGVAR/authorized_paths.conf`（cmd/config_callback 与 cmd/main 写入）
