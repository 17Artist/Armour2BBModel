# Armour2BBModel

将 [Armourer's Workshop](https://github.com/Armourers-Workshop/Armourers-Workshop) 时装皮肤文件（.armour）转换为 [Blockbench](https://www.blockbench.net/) 模型文件（.bbmodel）。

转换完全在浏览器本地通过 WebAssembly 运行，文件不会上传到任何服务器。

## 功能

- 支持 AW v1-v13 及 v20+ 格式
- 按模板骨骼输出，自动匹配时装类型
- 面剔除 + 方块合并（6 种策略取最优）
- 纹理图集逐像素绘制
- 翅膀动画自动生成
- 输出 Blockbench 5.0 格式

## 构建

需要 Go 1.21+。

```bash
# 编译 WASM
GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o web/static/convert.wasm ./cmd/wasm/
gzip -k -9 web/static/convert.wasm

# 编译静态服务器（可选）
go build -o armour2bbmodel .

# 启动
./armour2bbmodel
```

`web/static/` 目录可直接部署到任意静态托管。

## 项目结构

```
skin/           AW 皮肤文件解析
converter/      转换核心
bbmodel/        Blockbench 5.0 数据结构
cmd/wasm/       WebAssembly 入口
web/static/     前端静态文件
```

## 许可证

[CC-BY-NC-SA-3.0](https://creativecommons.org/licenses/by-nc-sa/3.0/)

皮肤文件格式解析参照 [Armourer's Workshop](https://github.com/Armourers-Workshop/Armourers-Workshop) 开源代码实现。


