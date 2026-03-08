# nmd 使用文档

本文档介绍 `nmd` 的核心使用方式、常见工作流和快捷键。

## 1. 启动应用

在项目根目录执行：

```bash
make dev
```

## 2. 界面结构

- 顶部工具栏：文件操作、视图切换、编辑格式化、主题/语言切换。
- 左侧边栏：大纲（Outline）和最近文件（Recent Files）。
- 中间区域：编辑区与预览区（支持 `Split / Edit / Preview` 模式）。
- 底部状态栏：自动保存状态、同步状态、光标位置、字数统计等。

## 3. 常用功能

### 3.1 文件操作

- `New`：新建文档
- `Open`：打开本地 Markdown
- `Save` / `Save As`
- `PDF`：导出 PDF

### 3.2 编辑与预览

- Markdown 实时预览
- 编辑/预览滚动同步（可开关）
- 格式化按钮：`Bold`、`Italic`、`H1`、`H2`、`Quote`、`List`、`Code`
- 拖拽或粘贴图片到编辑器

### 3.3 自动保存与错误管理

- 自动保存开关与保存间隔切换
- 自动保存失败后可：
  - `Retry`
  - 查看错误详情
  - 复制错误
  - 导出错误日志
  - 删除/批量删除/筛选删除（均带二次确认）
  - 撤销删除（Undo Delete）与重做删除（Redo Delete）

### 3.4 语言切换

- 顶部工具栏提供 `EN / 中文` 切换按钮
- 语言设置会自动持久化，下次启动沿用

## 4. 常用快捷键

- `Ctrl/Cmd + N`：新建
- `Ctrl/Cmd + O`：打开
- `Ctrl/Cmd + S`：保存
- `Ctrl/Cmd + Shift + S`：另存为
- `Ctrl/Cmd + K`：命令面板
- `Ctrl/Cmd + H`：查找替换面板
- `Ctrl/Cmd + L`：跳转到行
- `Ctrl/Cmd + B`：切换侧边栏
- `Ctrl/Cmd + 1/2/3`：分栏/仅编辑/仅预览
- `Ctrl/Cmd + Shift + A`：切换自动保存
- `Ctrl/Cmd + Shift + T`：切换自动保存间隔
- `Ctrl/Cmd + Shift + E`：显示自动保存错误
- `Ctrl/Cmd + Shift + C`：复制自动保存错误
- `Ctrl/Cmd + Shift + R`：撤销删除（错误历史）
- `Ctrl/Cmd + Shift + J`：重做删除（错误历史）
- `Ctrl/Cmd + Z`：撤销编辑
- `Ctrl/Cmd + Shift + Z` 或 `Ctrl/Cmd + Y`：重做编辑

## 5. 推荐工作流

1. `Open` 打开 Markdown 文档。
2. 在 `Split` 模式下编辑并观察预览。
3. 用边栏大纲快速跳转章节。
4. 完成后 `Save`，需要分享时导出 `PDF`。
5. 若出现自动保存异常，进入错误详情面板进行排查或导出日志。

## 6. 常见问题

### Q1: 命令面板被遮挡

已在样式层级中修复。若出现异常，重启 `make dev`。

### Q2: 无法“撤销撤销”（Redo）

使用：

- `Ctrl/Cmd + Shift + Z`
- 或 `Ctrl/Cmd + Y`

### Q3: 图片显示异常

- 优先使用拖拽/粘贴图片到编辑器
- 确认图片路径与文档路径关系正确
- 可查看底部状态栏中的图片状态提示
