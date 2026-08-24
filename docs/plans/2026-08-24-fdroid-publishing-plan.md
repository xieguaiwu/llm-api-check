# F-Droid 发布调查与安卓发表计划（2026-08-24）

> 结论先行：**安卓侧发布对象 = 姊妹仓库 pocket-llm-api-checker（Kotlin 原生 App，MIT）**。
> 本仓库（Go CLI）是它的数据层复刻，不参与安卓发布，但可作为 F-Droid 描述中的关联项目。
> 项目当前 100% 满足 F-Droid 硬性要求（FLOSS、依赖、权限、隐私），缺的是**发布工程化**：
> git tag、fastlane 元数据、fdroiddata MR。可复现构建可行性高（纯 Kotlin、无 R8、无 NDK）。

## 执行状态（2026-08-24 更新）

**Phase 1 已完成 ✅**（commit e1c1568 + tag v1.0.0 已推送）：
- fastlane 元数据（en-US + zh-CN 双语文案、icon.png 矢量精确渲染、占位截图×2、changelogs/1.txt）
- 本地 `assembleRelease` 通过（versionCode=1 / versionName=1.0.0，与 tag 对应）
- **可复现性实测通过**：双构建 SHA-256 一致（4b49d6c9…）——Verified 徽章路线随时可启用（首次发布前需生成 keystore）
- fdroiddata 草稿 `docs/fdroid/com.xieguiawu.apicheckers.yml`，Categories `System` 已对照官方 categories.yml 验证存在

**Phase 2 待用户**：①真机侧载 APK 截 2 张真实截图替换 fastlane 占位图 ②GitLab 账号 fork fdroiddata 提交 metadata MR（CI 自动验证）③合并后 24–48h 上线

**Phase 3 发版纪律**：bump versionCode/versionName → tag vX.Y.Z → 更新 changelogs/<versionCode>.txt

## 一、现状核查（2026-08-24 实测）

| 项 | CLI 仓库（当前目录） | 安卓仓库 pocket-llm-api-checker |
|---|---|---|
| 语言/栈 | Go 1.25，零第三方依赖 | Kotlin 2.0.21 + Compose (BOM 2024.10.00)、AGP 8.5.2、Gradle 8.9（wrapper 已提交） |
| 包名 | — | `com.xieguiawu.apicheckers`，minSdk 26 / targetSdk 35 |
| License | MIT ✓（LICENSE 文件） | MIT ✓ |
| 权限 | — | 仅 INTERNET；`usesCleartextTraffic=false`、`allowBackup=false` |
| 凭据 | `~/.config/llm-api-check/config.json` chmod 0600 | Android Keystore AES-GCM（`api_checkers_master` alias） |
| 仓库 | github.com/xieguaiwu/llm-api-check（PUBLIC） | github.com/xieguaiwu/pocket-llm-api-checker（PUBLIC，issues 开） |
| git tag | 无（CLI 不需要） | **无 tag** ← 发布阻塞项 |
| fastlane 元数据 | — | **无** ← 发布阻塞项 |
| CI | 无 | 无 |

## 二、F-Droid 发表限制审查

### ✅ 已满足（官方 Inclusion Policy + Quick Start 清单）
1. **FLOSS License**：MIT，LICENSE 文件在仓库根 ✓
2. **仅 FOSS 依赖**：androidx / okhttp / kotlinx-serialization，全部来自 `google()` + `mavenCentral()`，无 GMS/Firebase/广告/统计/支付 SDK ✓
3. **无专有二进制**：纯 Kotlin，无 NDK、无 jar 入库 ✓
4. **fdroidserver 可构建**：标准 Gradle 项目，wrapper 已提交，AGP 8.5.2 + JDK17 在 F-Droid buildserver（Debian 容器）支持范围 ✓
5. **隐私面干净**：单权限、禁明文流量、禁备份、Keystore 加密、无遥测 ✓
6. **版本信息位置标准**：versionCode/versionName 在 `app/build.gradle.kts` 的 android 块 → `UpdateCheckMode: Tags` 可直接正则提取 ✓

### ⚠️ 需要注意（非阻止，但 review 必谈）
1. **NonFreeNet 反特性必然触发**：app「promotes or depends entirely on a proprietary network service」——DeepSeek API 与 opencode.ai 均为专有服务，且本 app 核心功能完全依赖它们。**不阻止收录**（仅客户端页面显示警告徽章），但 metadata 必须声明 `AntiFeatures: NonFreeNet` 并在 full_description 说明。
2. **页面解析脆弱性**：Zen billing 靠正则解析 SolidJS SSR HTML，reviewer 会注意到；建议描述里写明「数据来自官方页面/API，页面改版可能影响功能」。
3. **通用名 "API Checkers"**：F-Droid 现有包中无冲突（可保留），但建议 metadata 里用 AutoName/描述区分。

### ❌ 缺失（阻止收录，必须补齐）
1. **git 版本 tag**：零 tag。F-Droid 每个 Builds 块必须指定 commit/tag；autoupdate 依赖 tag 命名纪律（`v<versionName>` 或 `v<versionCode>`）。
2. **fastlane/Triple-T 元数据**（官方措辞 "should always be added before inclusion"）：
   - `fastlane/metadata/android/en-US/short_description.txt`（<80 字符，无句点）
   - `fastlane/metadata/android/en-US/full_description.txt`
   - `fastlane/metadata/android/en-US/images/icon.png`（需从 adaptive icon 导出 PNG）
   - `fastlane/metadata/android/en-US/images/phoneScreenshots/1.png`、`2.png`
   - `fastlane/metadata/android/en-US/changelogs/<versionCode>.txt`（≤500 字符）
   - 可加 `zh-CN` 本地化目录
3. **fdroiddata 提交**（唯一正式入口）：fork `gitlab.com/fdroid/fdroiddata` → `metadata/com.xieguiawu.apicheckers.yml` → MR。GitLab CI 自动跑 lint + 构建验证。

### 🔑 可复现构建评估（强烈推荐，非强制）
- **可行性高**：纯 Kotlin、`isMinifyEnabled=false`（无 R8 非确定性）、无 NDK/PNG crunch/baseline.prof → 大概率开箱可复现。
- AGP 8.3+ 默认生成的 `META-INF/version-control-info.textproto` 在 tag 处干净树构建即确定（local_root_path 写的是 `$PROJECT_DIR` 占位符，不泄露路径）。
- 收益：metadata 加 `Binaries:` + `AllowedAPKSigningKeys:` → F-Droid「Verified」徽章 + 用户可同时从 GitHub Releases 更新；签名不可中途更换（Android 限制），**新应用必须一开始就决定**。

## 三、实施计划

### Phase 1 — repo 侧准备（我可直接实施）
1. 打 tag：`git tag v1.0.0`（versionCode=1 / versionName=1.0.0 现状即可，无需改代码）
2. 新增 fastlane 元数据（en-US + zh-CN：short/full description、icon.png 从 vector 导出、2 张 phoneScreenshots、changelogs/1.txt）
3. 可复现构建验证脚本：同一 commit 本地构建两次 + `diffoscope`/`sha256sum` 对比；或 CI 双 runner 对比
4. （可选）GitHub Actions：tag 触发 `assembleRelease` → 签名（用户 keystore）→ GitHub Releases，作为 F-Droid 的 Binaries 来源
5. README/CONTEXT 补 F-Droid 徽章与发布说明

### Phase 2 — fdroiddata 提交（需用户 GitLab 账号，我可起草文件）
1. fork fdroiddata，新建 `metadata/com.xieguiawu.apicheckers.yml`：
   - `Categories: System`（fdroiddata categories.yml 现有类别）
   - `License: MIT`；`SourceCode` / `IssueTracker` / `Repo` 指 GitHub
   - `Builds: [{versionName: 1.0.0, versionCode: 1, commit: v1.0.0}]`
   - `AutoUpdateMode: Version`；`UpdateCheckMode: Tags`
   - `AntiFeatures: NonFreeNet`
   - 若走可复现：`Binaries: https://github.com/xieguaiwu/pocket-llm-api-checker/releases/download/v%v/...` + `AllowedAPKSigningKeys: <SHA-256>`
2. 本地 `fdroid lint` 校验（或直接推分支让 GitLab CI 跑）
3. 提交 MR → 维护者 review → 合并后 24–48h 出现在主仓库

### Phase 3 — 发布后维护纪律
- 每次发版：bump versionCode/versionName → `git tag vX.Y.Z` → 更新 `changelogs/<versionCode>.txt`
- F-Droid checkupdates 每日自动发现新 tag
- 保持可复现：从 tag 干净树构建；CI 用固定 build-tools 34 的 apksigner（35+ 有 apksigcopier 兼容问题，若走 Verified 需注意）

## 四、风险清单
- NonFreeNet 徽章影响观感（不阻止收录）——可接受，如实声明
- billing HTML 解析改版即坏——已在 README 声明；F-Droid 侧无责
- 若选自有签名：keystore 丢失 = 无法更新（需妥善备份）；**决策窗口在首次发布前**
- fdroiddata review 周期不可控（社区排队），做好 1–4 周预期
