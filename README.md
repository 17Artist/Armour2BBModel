# Armour2BBModel

将 [Armourer's Workshop](https://github.com/Armourers-Workshop/Armourers-Workshop) 的体素时装转换为 [Blockbench](https://www.blockbench.net/) `.bbmodel`。支持 `.armour` / `.awsk` 文件。

浏览器转换在本机 Web Worker 中运行，时装内容不会上传。静态工作台提供五类预览与单类下载、来源部件检查、纹理图集和实际转换统计；也提供命令行转换器。

## 使用

Windows 上运行 `armour2bbmodel.exe`，打开 `http://localhost:8080`，选择或拖入自己的时装文件。

转换结果按 ArcartX 的**头、上半身、下半身、鞋、装饰**五类显示。每个方块只属一个类别，分类可单独预览并下载；源部件名称、注册类型和装备序号保留在折叠详情中，可逐源件检查。选中分类会自动适配视角。模型预览支持拖动、滚轮、方向键和按钮控制，图集页可检查纹理。单文件上限为 64 MiB，转换可以取消或失败后重试；超过 80,000 个输出块时保留下载、跳过三维预览。


命令行使用相同的转换核心：

```powershell
./armour-convert.exe -input skin.armour -output model.bbmodel
# 或从源码运行
go run ./cmd/convert -input skin.armour -output model.bbmodel
# 保留原始重叠面以便拆分附件编辑
./armour-convert.exe -input skin.armour -output model.bbmodel -keep-coincident-faces
# 单独导出 ArcartX 上半身（也支持 HEAD/LEGS/FEET/DECORATION）
./armour-convert.exe -input skin.armour -output upper.bbmodel -slot BODY
```

## 转换范围

- AW v1–13 和 v20–25 的绘制体素格式，含旧数字部件 ID、gzip 文件/数据块、调色板、四种体素材质和几何选择器。
- 按注册类型映射五类，并分别记录来源信息与目标槽；保留累积变换。静态源树中的显式注册部件按对应目标骨分组，advanced 部件继承父来源，动态翼子树保留装饰运动链；裙分两侧腿，无标准宿主对应的部件和物品保留为装饰。分片前后逐格守恒，武器、裙和身体附件不会被骨骼名单过滤掉。
- 保留普通旋转、平移、缩放、枢轴与高级人体的躯干/前臂/小腿关节偏移。重复坐标分层处理，避免几何相互覆盖。
- 相邻面剔除，玻璃 alpha 127，空白画笔面隐藏，合并面局部遮挡使用透明纹理遮罩。导出隐藏面显式设置 `texture: null`，符合 Blockbench 的隐藏面读取规则。
- 四十八方向候选、按材质连通区域独立选优、整面合并、小区域有预算的精确覆盖搜索，以及相邻三块重切为两块、相邻四块重切为三块。
- 输出 Blockbench 5.0 文件；按套装 `partIndexs` 读取每个子装备的独立参数、marker 轴和枢轴。固定角度写入静态姿态；有实际运动的翼按独立 UUID 生成 idle，EASE 用 32 段线性采样，LINEAR 保留周期回跳。

加密文件、导入的 texture/mesh 模型、镜像或形成剪切的变换明确报错。源 ANIM 动画与玩家彩绘贴图没有导出，文件含这些数据时界面显示对应说明。当前相邻面剔除保留封闭空腔内面，与 AW 从外部空气出发的完整剔除还有区别。


## 构建

需要 Go 1.26.1+（与 `go.mod` 一致）。Go 的 WASM 文件与 `wasm_exec.js` 必须来自相同工具链。

```powershell
./scripts/build.ps1
```

脚本同步构建 `web/static/convert.wasm`、gzip 版本及配套 runtime，再构建静态服务器和命令行程序。Linux/macOS 可按以下方式构建：

```sh
GOOS=js GOARCH=wasm go build -buildvcs=false -trimpath -ldflags="-s -w" -o web/static/convert.wasm ./cmd/wasm
# 同步所用 Go 工具链的 runtime。
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/static/wasm_exec.js
gzip -n -9 -c web/static/convert.wasm > web/static/convert.wasm.gz
go build -o armour2bbmodel .
go build -o armour-convert ./cmd/convert
```

`web/static/` 可部署到静态托管。

## 构建与静态检查

```powershell
go build ./...
go vet ./...
```



## 目录与许可证

`skin/` 解析格式；`converter/` 转换与合并；`bbmodel/` 输出结构；`cmd/wasm/` / `cmd/convert/` 提供入口；`web/static/` 是工作台；`scripts/build.ps1` 构建发布文件；`docs/` 保留格式契约与研究记录；`design-system/` 记录界面规范。

项目许可证与署名见 [LICENSE](LICENSE)，采用 [CC-BY-NC-SA-3.0](https://creativecommons.org/licenses/by-nc-sa/3.0/)。Go 运行时与 UUID 依赖的上游许可见 [第三方声明](THIRD_PARTY_NOTICES.md)。工作台不提供内置时装素材，请导入自己的文件。


