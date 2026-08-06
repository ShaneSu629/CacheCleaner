<#
╔══════════════════════════════════════════════════════════════════╗
║            智能缓存清理工具  Smart Cache Cleaner                    ║
║  自动扫描当前用户目录下的 AI 缓存、软件缓存、临时文件                    ║
║  智能识别 → 风险分级 → 交互选择 → 安全清理                            ║
╚══════════════════════════════════════════════════════════════════╝
#>

#Requires -Version 5.1

# ============================================================
# 配置区域 - 你可以在下面修改默认设置
# ============================================================
$Script:Config = @{
    # 扫描的目标目录（默认自动检测当前用户目录，可改为 "C:\" 扫描整个C盘）
    UserHome       = $env:USERPROFILE
    # 配置文件路径
    ConfigFile     = "$PSScriptRoot\config\CacheCleaner.config.json"
    # 排除的目录名（不扫描这些名字的目录）
    ExcludeDirs    = @(
        'Windows', 'System32', 'Program Files', 'Program Files (x86)'
        'ProgramData', 'Common Files', '.git', 'node_modules', 'Microsoft.NET'
    )
    # 缓存关键词（目录名包含这些关键词会被智能识别为缓存）
    CacheKeywords  = @(
        'cache', 'Cache', 'CACHE', '缓存',
        'temp', 'Temp', 'TEMP', 'tmp', 'Tmp', '临时',
        'log', 'Log', 'LOG', '日志',
        'backup', 'Backup', '备份',
        'history', 'History', '历史',
        'recent', 'Recent', '最近',
        'trash', 'Trash', '回收站',
        'download', 'Download', '下载',
        'crash', 'Crash', '崩溃',
        'dump', 'Dump', '转储',
        'prefetch', 'Prefetch',
        'thumb', 'Thumb', '缩略图',
        'recycle', 'Recycle',
        'old', 'Old', '旧的',
        'backup', 'bak',
        'data', 'Data'  # 有些 AppData 下的 data 目录可清理
    )
    # 缓存文件扩展名
    CacheExtensions = @(
        '.log', '.tmp', '.temp', '.bak', '.old', '.cache'
        '.dmp', '.dump', '.etl', '.blf', '.regtrans-ms'
    )
    # 大文件阈值（MB）- 超过此大小的文件会单独列出
    LargeFileThresholdMB = 50
    # 天数阈值 - 超过此天数未访问的文件视为"可清理的旧文件"
    OldFileDays = 30
}

# ============================================================
# 已知缓存数据库 - 按风险等级分类
# 智能识别会优先匹配这些已知路径
# ============================================================
$Script:KnownCaches = @(
    # ─── AI 缓存（重点清理对象） ───
    @{Path="\.cursor";        Category="AI缓存"; Risk="Safe"; Desc="Cursor AI 编辑器缓存(代码补全历史等)"},
    @{Path="\.cursor\extensions"; Category="AI缓存"; Risk="Safe"; Desc="Cursor AI 扩展缓存"},
    @{Path="\.cursor\machineid"; Category="AI缓存"; Risk="Safe"; Desc="Cursor AI 机器标识(可重新生成)"},
    @{Path="\.cursor\CachedData"; Category="AI缓存"; Risk="Safe"; Desc="Cursor AI 缓存数据"},
    @{Path="\AppData\Roaming\Cursor\Cache"; Category="AI缓存"; Risk="Safe"; Desc="Cursor AI 全局缓存"},
    @{Path="\AppData\Roaming\Cursor\CachedData"; Category="AI缓存"; Risk="Safe"; Desc="Cursor AI 全局缓存数据"},
    @{Path="\AppData\Roaming\Cursor\Code Cache"; Category="AI缓存"; Risk="Safe"; Desc="Cursor AI 代码缓存"},
    @{Path="\AppData\Local\Trae\Cache"; Category="AI缓存"; Risk="Safe"; Desc="Trae AI 编辑器缓存"},
    @{Path="\AppData\Local\Trae\CachedData"; Category="AI缓存"; Risk="Safe"; Desc="Trae AI 缓存数据"},
    @{Path="\AppData\Local\Trae\Code Cache"; Category="AI缓存"; Risk="Safe"; Desc="Trae AI 代码缓存"},
    @{Path="\AppData\Roaming\Code\Cache"; Category="AI缓存"; Risk="Safe"; Desc="VS Code 缓存(含AI扩展)"},
    @{Path="\AppData\Roaming\Code\CachedData"; Category="AI缓存"; Risk="Safe"; Desc="VS Code 缓存数据(含AI扩展)"},
    @{Path="\AppData\Roaming\Code\User\workspaceStorage"; Category="AI缓存"; Risk="Caution"; Desc="VS Code 工作区存储(重新打开项目会恢复)"},
    @{Path="\AppData\Roaming\Code\Code Cache"; Category="AI缓存"; Risk="Safe"; Desc="VS Code 代码缓存"},
    @{Path="\AppData\Local\GitHubCopilot"; Category="AI缓存"; Risk="Safe"; Desc="GitHub Copilot AI 缓存"},
    @{Path="\AppData\Roaming\GitHub Copilot"; Category="AI缓存"; Risk="Caution"; Desc="GitHub Copilot 配置(清理后需重新登录)"},
    @{Path="\AppData\Local\Microsoft\VSComponents\Cache"; Category="AI缓存"; Risk="Safe"; Desc="VS 组件缓存(含AI组件)"},
    @{Path="\.cache";           Category="AI缓存"; Risk="Safe"; Desc="通用缓存目录(含AI模型缓存)"},
    @{Path="\.cache\huggingface"; Category="AI缓存"; Risk="Caution"; Desc="HuggingFace AI 模型缓存(可能较大)"},
    @{Path="\.cache\torch";     Category="AI缓存"; Risk="Safe"; Desc="PyTorch AI 模型缓存"},
    @{Path="\.cache\pip";       Category="AI缓存"; Risk="Safe"; Desc="pip AI 包缓存"},
    @{Path="\AppData\Local\ollama\models"; Category="AI缓存"; Risk="Review"; Desc="Ollama AI 本地模型(下载大模型,谨慎清理)"},
    @{Path="\AppData\Local\LM Studio\models"; Category="AI缓存"; Risk="Review"; Desc="LM Studio AI 模型(谨慎清理)"},
    @{Path="\AppData\Local\CachedData"; Category="AI缓存"; Risk="Safe"; Desc="系统 AI 缓存数据"},
    @{Path="\.vscode\CachedData"; Category="AI缓存"; Risk="Safe"; Desc="VS Code 缓存数据(含AI扩展)"},
    @{Path="\.vscode\extensions\.cache"; Category="AI缓存"; Risk="Safe"; Desc="VS Code 扩展缓存(含AI扩展)"},
    @{Path="\.vscode\extensions\*\\.cache"; Category="AI缓存"; Risk="Safe"; Desc="VS Code 各扩展缓存"},

    # ─── 浏览器缓存（安全） ───
    @{Path="\AppData\Local\Google\Chrome\User Data\Default\Cache"; Category="浏览器缓存"; Risk="Safe"; Desc="Chrome 浏览器缓存"},
    @{Path="\AppData\Local\Google\Chrome\User Data\Default\Code Cache"; Category="浏览器缓存"; Risk="Safe"; Desc="Chrome 代码缓存"},
    @{Path="\AppData\Local\Google\Chrome\User Data\Default\Service Worker\CacheStorage"; Category="浏览器缓存"; Risk="Safe"; Desc="Chrome Service Worker 缓存"},
    @{Path="\AppData\Local\Microsoft\Edge\User Data\Default\Cache"; Category="浏览器缓存"; Risk="Safe"; Desc="Edge 浏览器缓存"},
    @{Path="\AppData\Local\Microsoft\Edge\User Data\Default\Code Cache"; Category="浏览器缓存"; Risk="Safe"; Desc="Edge 代码缓存"},
    @{Path="\AppData\Local\Microsoft\Edge\User Data\Default\Service Worker\CacheStorage"; Category="浏览器缓存"; Risk="Safe"; Desc="Edge Service Worker 缓存"},
    @{Path="\AppData\Local\Microsoft\Windows\INetCache"; Category="浏览器缓存"; Risk="Safe"; Desc="Internet 临时文件"},
    @{Path="\AppData\Local\Microsoft\Windows\WebCache"; Category="浏览器缓存"; Risk="Caution"; Desc="Web 缓存(可能影响浏览器登录状态)"},

    # ─── 开发工具缓存（安全） ───
    @{Path="\AppData\Local\npm-cache"; Category="开发工具缓存"; Risk="Safe"; Desc="npm 包缓存"},
    @{Path="\AppData\Local\pip\cache"; Category="开发工具缓存"; Risk="Safe"; Desc="pip 包缓存"},
    @{Path="\AppData\Local\pip\Cache"; Category="开发工具缓存"; Risk="Safe"; Desc="pip 缓存"},
    @{Path="\.npm";              Category="开发工具缓存"; Risk="Safe"; Desc="npm 全局缓存"},
    @{Path="\.yarn\cache";       Category="开发工具缓存"; Risk="Safe"; Desc="Yarn 包缓存"},
    @{Path="\AppData\Local\Yarn\Cache"; Category="开发工具缓存"; Risk="Safe"; Desc="Yarn 全局缓存"},
    @{Path="\.gradle\caches";    Category="开发工具缓存"; Risk="Caution"; Desc="Gradle 构建缓存(清理后需重新构建)"},
    @{Path="\.gradle\wrapper\dists"; Category="开发工具缓存"; Risk="Safe"; Desc="Gradle Wrapper 分发包"},
    @{Path="\.m2\repository";    Category="开发工具缓存"; Risk="Caution"; Desc="Maven 本地仓库(清理后需重新下载依赖)"},
    @{Path="\AppData\Local\Composer"; Category="开发工具缓存"; Risk="Safe"; Desc="Composer(PHP) 缓存"},
    @{Path="\AppData\Local\main.kts.compiled.cache"; Category="开发工具缓存"; Risk="Safe"; Desc="Kotlin 脚本编译缓存"},

    # ─── IDE 缓存（安全） ───
    @{Path="\AppData\Local\JetBrains"; Category="IDE缓存"; Risk="Caution"; Desc="JetBrains IDE 缓存(清理后需重新索引)"},
    @{Path="\AppData\Roaming\JetBrains\*\caches"; Category="IDE缓存"; Risk="Safe"; Desc="JetBrains IDE 缓存数据"},
    @{Path="\AppData\Roaming\JetBrains\*\logs"; Category="IDE缓存"; Risk="Safe"; Desc="JetBrains IDE 日志"},

    # ─── Windows 系统缓存（安全） ───
    @{Path="\AppData\Local\Temp";          Category="系统临时文件"; Risk="Safe"; Desc="Windows 临时文件"},
    @{Path="\AppData\Local\Microsoft\Windows\Explorer"; Category="系统临时文件"; Risk="Safe"; Desc="资源管理器缩略图缓存"},
    @{Path="\AppData\Local\D3DSCache";     Category="系统临时文件"; Risk="Safe"; Desc="Direct3D 着色器缓存"},
    @{Path="\AppData\Local\Microsoft\Windows\Caches"; Category="系统临时文件"; Risk="Caution"; Desc="系统图标缓存(清理后需重启)"},
    @{Path="\AppData\Local\IconCache.db";   Category="系统临时文件"; Risk="Safe"; Desc="图标缓存文件(单文件)"},

    # ─── 软件缓存（安全） ───
    @{Path="\AppData\Local\Microsoft\OneDrive\logs"; Category="软件缓存"; Risk="Safe"; Desc="OneDrive 日志"},
    @{Path="\AppData\Local\Microsoft\Windows Mail\Temp"; Category="软件缓存"; Risk="Safe"; Desc="Windows Mail 临时文件"},
    @{Path="\AppData\Local\Packages\*\AC\INetCache"; Category="软件缓存"; Risk="Safe"; Desc="UWP 应用缓存"},
    @{Path="\AppData\Local\Package Cache";  Category="软件缓存"; Risk="Safe"; Desc="安装包缓存"},
    @{Path="\AppData\Local\apipost-updater"; Category="软件缓存"; Risk="Safe"; Desc="ApiPost 更新缓存"},

    # ─── 需要谨慎清理 ───
    @{Path="\AppData\Local\Microsoft\OneDrive\settings"; Category="需谨慎"; Risk="Review"; Desc="OneDrive 设置(可能丢失登录信息)"},
    @{Path="\AppData\Roaming\Microsoft\Teams\Cache"; Category="需谨慎"; Risk="Caution"; Desc="Teams 缓存(需重新登录)"},
    @{Path="\AppData\Roaming\Slack\Cache"; Category="需谨慎"; Risk="Caution"; Desc="Slack 缓存(需重新登录)"}
)

# ============================================================
# 辅助函数
# ============================================================

# 格式化文件大小（人类可读）
function Format-FileSize {
    param([long]$Bytes)
    if ($Bytes -ge 1TB) { return "{0:N2} TB" -f ($Bytes / 1TB) }
    if ($Bytes -ge 1GB) { return "{0:N2} GB" -f ($Bytes / 1GB) }
    if ($Bytes -ge 1MB) { return "{0:N2} MB" -f ($Bytes / 1MB) }
    if ($Bytes -ge 1KB) { return "{0:N2} KB" -f ($Bytes / 1KB) }
    return "$Bytes B"
}

# 获取目录大小（递归）
function Get-DirectorySize {
    param([string]$Path)
    if (-not (Test-Path $Path)) { return 0 }
    try {
        $files = Get-ChildItem -Path $Path -Recurse -File -ErrorAction SilentlyContinue -Force
        return ($files | Measure-Object -Property Length -Sum -ErrorAction SilentlyContinue).Sum
    } catch { return 0 }
}

# 获取目录的最后访问时间
function Get-DirectoryLastAccess {
    param([string]$Path)
    if (-not (Test-Path $Path)) { return $null }
    try {
        $dir = Get-Item -Path $Path -ErrorAction SilentlyContinue
        return $dir.LastAccessTime
    } catch { return $null }
}

# 获取目录的文件数量
function Get-DirectoryFileCount {
    param([string]$Path)
    if (-not (Test-Path $Path)) { return 0 }
    try {
        return (Get-ChildItem -Path $Path -Recurse -File -ErrorAction SilentlyContinue -Force | Measure-Object).Count
    } catch { return 0 }
}

# 写彩色输出
function Write-Color {
    param(
        [string]$Text,
        [string]$Color = "White",
        [string]$BackColor = $null
    )
    if ($BackColor) {
        Write-Host $Text -ForegroundColor $Color -BackgroundColor $BackColor
    } else {
        Write-Host $Text -ForegroundColor $Color
    }
}

# 写标题
function Write-Header {
    param([string]$Title)
    $width = Get-ConsoleWidth
    Write-Color ("=" * $width) -Color "Cyan"
    Write-Color (Center-Text -Text " $Title " -Width $width) -Color "Yellow"
    Write-Color ("=" * $width) -Color "Cyan"
}

# 写分隔线
function Write-Separator {
    param([string]$Char = "-")
    $width = Get-ConsoleWidth
    Write-Color (Repeat-Char -Char $Char -Count $width) -Color "DarkGray"
}

# 暂停等待按键
function Wait-KeyPress {
    Write-Host ""
    Write-Host "按任意键继续..." -ForegroundColor DarkGray -NoNewline
    $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
    Write-Host ""
}

# 获取文本实际显示宽度（中文字符占2列）
function Get-DisplayWidth {
    param([string]$Text)
    $width = 0
    foreach ($char in $Text.ToCharArray()) {
        if ([int]$char -gt 127) { $width += 2 } else { $width += 1 }
    }
    return $width
}

# 获取控制台宽度（自适应），最小宽度 60，最大宽度 120
function Get-ConsoleWidth {
    $minWidth = 60
    $maxWidth = 120
    try {
        $w = $Host.UI.RawUI.WindowSize.Width
        if ($w -lt $minWidth) { return $minWidth }
        if ($w -gt $maxWidth) { return $maxWidth }
        return $w
    } catch { return $minWidth }
}

# 居中文本（带左右填充，支持中文字符）
function Center-Text {
    param([string]$Text, [int]$Width)
    $displayWidth = Get-DisplayWidth -Text $Text
    $pad = [math]::Max(0, [math]::Floor(($Width - $displayWidth) / 2))
    $rightPad = $Width - $pad - $displayWidth
    if ($rightPad -lt 0) { $rightPad = 0 }
    return (" " * $pad) + $Text + (" " * $rightPad)
}

# 重复字符，支持中文字符
function Repeat-Char {
    param([string]$Char, [int]$Count)
    if ($Count -le 0) { return "" }
    return "$Char" * $Count
}

# ============================================================
# 智能扫描 - 发现已知缓存目录
# ============================================================
function Find-KnownCaches {
    Write-Host "`n正在扫描已知缓存目录..." -ForegroundColor Yellow
    
    $results = @()
    $total = $KnownCaches.Count
    $current = 0
    
    foreach ($cache in $KnownCaches) {
        $current++
        $fullPath = $Config.UserHome + $cache.Path
        
        # 支持通配符路径（如 JetBrains\*）
        if ($fullPath.Contains('*')) {
            $matchedDirs = Get-ChildItem -Path ($Config.UserHome + "\AppData\Roaming\JetBrains") -Directory -ErrorAction SilentlyContinue
            foreach ($matchedDir in $matchedDirs) {
                $subPath = $matchedDir.FullName + "\caches"
                if (Test-Path $subPath) {
                    $size = Get-DirectorySize -Path $subPath
                    $results += [PSCustomObject]@{
                        Path       = $subPath
                        ShortPath  = "AppData\Roaming\JetBrains\$($matchedDir.Name)\caches"
                        Category   = $cache.Category
                        Risk       = $cache.Risk
                        Desc       = $cache.Desc
                        Size       = $size
                        SizeStr    = Format-FileSize -Bytes $size
                        FileCount  = Get-DirectoryFileCount -Path $subPath
                        LastAccess = Get-DirectoryLastAccess -Path $subPath
                    }
                }
            }
            continue
        }
        
        if (Test-Path $fullPath) {
            $size = Get-DirectorySize -Path $fullPath
            $results += [PSCustomObject]@{
                Path       = $fullPath
                ShortPath  = $cache.Path.TrimStart('\')
                Category   = $cache.Category
                Risk       = $cache.Risk
                Desc       = $cache.Desc
                Size       = $size
                SizeStr    = Format-FileSize -Bytes $size
                FileCount  = Get-DirectoryFileCount -Path $fullPath
                LastAccess = Get-DirectoryLastAccess -Path $fullPath
            }
        }
        
        # 进度指示
        if ($current % 10 -eq 0 -or $current -eq $total) {
            Write-Host "  进度: $current/$total" -NoNewline -ForegroundColor DarkGray
            Write-Host "`r" -NoNewline
        }
    }
    
    return $results
}

# ============================================================
# 智能发现 - 通过关键词自动发现缓存目录
# ============================================================
function Auto-DiscoverCaches {
    Write-Host "正在智能发现缓存目录..." -ForegroundColor Yellow
    
    $discovered = @()
    $searchPaths = @(
        "$($Config.UserHome)\AppData\Local",
        "$($Config.UserHome)\AppData\Roaming",
        "$($Config.UserHome)\AppData\LocalLow"
    )
    
    $alreadyKnown = $KnownCaches | ForEach-Object { $Config.UserHome + $_.Path }
    
    $maxDepth = 3
    $checked = 0
    
    foreach ($root in $searchPaths) {
        if (-not (Test-Path $root)) { continue }
        
        try {
            $dirs = Get-ChildItem -Path $root -Directory -Recurse -Depth $maxDepth -ErrorAction SilentlyContinue -Force | `
                Where-Object {
                    $skip = $false
                    foreach ($ex in $Config.ExcludeDirs) {
                        if ($_.FullName -match [regex]::Escape($ex)) { $skip = $true; break }
                    }
                    -not $skip
                }
            
            foreach ($dir in $dirs) {
                $checked++
                $name = $dir.Name
                $fullName = $dir.FullName
                
                # 跳过已知目录
                $isKnown = $false
                foreach ($known in $alreadyKnown) {
                    if ($fullName -eq $known -or $fullName.StartsWith($known + '\')) { $isKnown = $true; break }
                }
                if ($isKnown) { continue }
                
                # 跳过深层子目录（只检查3层以内）
                $relativePath = $fullName.Substring($Config.UserHome.Length).TrimStart('\')
                $depth = ($relativePath.Split('\') | Measure-Object).Count
                if ($depth -gt 3) { continue }
                
                # 匹配关键词
                $matched = $false
                foreach ($keyword in $Config.CacheKeywords) {
                    if ($name -match [regex]::Escape($keyword)) {
                        $matched = $true
                        break
                    }
                }
                
                # 额外检查：目录中是否有大量小文件（缓存特征）
                if (-not $matched) {
                    try {
                        $files = Get-ChildItem -Path $fullName -File -ErrorAction SilentlyContinue -Force
                        $extGroups = $files | Group-Object -Property Extension
                        $cacheFileCount = 0
                        foreach ($ext in $Config.CacheExtensions) {
                            $match = $extGroups | Where-Object { $_.Name -eq $ext }
                            if ($match) { $cacheFileCount += $match.Count }
                        }
                        # 如果缓存文件占比超过50%，也视为缓存目录
                        if ($files.Count -gt 10 -and $cacheFileCount -gt 0 -and ($cacheFileCount / $files.Count) -gt 0.5) {
                            $matched = $true
                        }
                    } catch {}
                }
                
                if ($matched) {
                    $size = Get-DirectorySize -Path $fullName
                    # 跳过大小为0的目录
                    if ($size -eq 0) { continue }
                    
                    # 判断风险等级
                    $risk = "Caution"
                    $lowerName = $name.ToLower()
                    if ($lowerName -match 'cache|temp|tmp|log|backup') { $risk = "Safe" }
                    
                    $discovered += [PSCustomObject]@{
                        Path       = $fullName
                        ShortPath  = $relativePath
                        Category   = "自动发现"
                        Risk       = $risk
                        Desc       = "自动识别的缓存目录"
                        Size       = $size
                        SizeStr    = Format-FileSize -Bytes $size
                        FileCount  = Get-DirectoryFileCount -Path $fullName
                        LastAccess = Get-DirectoryLastAccess -Path $fullName
                    }
                }
                
                # 进度指示
                if ($checked % 200 -eq 0) {
                    Write-Host "  已扫描 $checked 个目录..." -NoNewline -ForegroundColor DarkGray
                    Write-Host "`r" -NoNewline
                }
            }
        } catch {}
    }
    
    # 按大小降序排列，取前50个
    $discovered = $discovered | Sort-Object -Property Size -Descending | Select-Object -First 50
    Write-Host "  扫描完成，共检查 $checked 个目录" -ForegroundColor DarkGray
    return $discovered
}

# ============================================================
# 智能发现大文件
# ============================================================
function Find-LargeFiles {
    Write-Host "正在扫描大文件..." -ForegroundColor Yellow
    
    $largeFiles = @()
    $searchPaths = @(
        "$($Config.UserHome)\AppData\Local\Temp",
        "$($Config.UserHome)\AppData\Local\Microsoft\Windows\INetCache"
    )
    
    $thresholdBytes = $Config.LargeFileThresholdMB * 1MB
    
    foreach ($path in $searchPaths) {
        if (-not (Test-Path $path)) { continue }
        try {
            $files = Get-ChildItem -Path $path -Recurse -File -ErrorAction SilentlyContinue -Force | `
                Where-Object { $_.Length -ge $thresholdBytes }
            
            foreach ($file in $files) {
                $largeFiles += [PSCustomObject]@{
                    Path       = $file.FullName
                    ShortPath  = $file.FullName.Substring($Config.UserHome.Length).TrimStart('\')
                    Size       = $file.Length
                    SizeStr    = Format-FileSize -Bytes $file.Length
                    LastAccess = $file.LastAccessTime
                    DaysOld    = [math]::Round(((Get-Date) - $file.LastAccessTime).TotalDays, 1)
                }
            }
        } catch {}
    }
    
    return ($largeFiles | Sort-Object -Property Size -Descending | Select-Object -First 20)
}

# ============================================================
# 清理操作
# ============================================================
function Invoke-CleanCache {
    param(
        [array]$Items,
        [string]$Mode = "normal" # normal 或 force
    )
    
    if ($Items.Count -eq 0) {
        Write-Color "  没有需要清理的项目。" -Color "Yellow"
        return 0
    }
    
    $totalFreed = 0
    $cleanedCount = 0
    $failCount = 0
    
    foreach ($item in $Items) {
        $path = $item.Path
        $isFile = $item.PSObject.Properties.Name -contains "DaysOld"
        
        if (-not (Test-Path $path)) {
            Write-Color "  [跳过] 不存在: $($item.ShortPath)" -Color "DarkGray"
            continue
        }
        
        # 检查文件是否被占用
        $locked = $false
        if ($isFile) {
            try {
                $stream = [System.IO.File]::Open($path, 'Open', 'Read', 'None')
                $stream.Close()
            } catch {
                $locked = $true
            }
        }
        
        if ($locked) {
            Write-Color "  [跳过] 文件被占用: $($item.ShortPath)" -Color "DarkYellow"
            continue
        }
        
        try {
            $size = $item.Size
            if ($isFile) {
                Remove-Item -Path $path -Force -ErrorAction Stop
            } else {
                # 清理目录内容（保留目录本身）
                Get-ChildItem -Path $path -Recurse -Force -ErrorAction SilentlyContinue | Remove-Item -Force -Recurse -ErrorAction SilentlyContinue
            }
            $totalFreed += $size
            $cleanedCount++
            Write-Color "  [清理] $($item.ShortPath)  ($(Format-FileSize -Bytes $size))" -Color "Green"
        } catch {
            Write-Color "  [失败] $($item.ShortPath) - $($_.Exception.Message)" -Color "Red"
            $failCount++
        }
    }
    
    Write-Color "`n  清理完成: $cleanedCount 项成功, $failCount 项失败" -Color "Cyan"
    Write-Color "  释放空间: $(Format-FileSize -Bytes $totalFreed)" -Color "Green"
    return $totalFreed
}

# ============================================================
# 加载/保存配置（自定义目录）
# ============================================================
function Load-CustomConfig {
    $configPath = $Config.ConfigFile
    if (Test-Path $configPath) {
        try {
            $config = Get-Content -Path $configPath -Raw -Encoding UTF8 | ConvertFrom-Json
            # 展开环境变量（如 %USERPROFILE%）
            if ($config.CustomDirs) {
                for ($i = 0; $i -lt $config.CustomDirs.Count; $i++) {
                    $config.CustomDirs[$i] = [Environment]::ExpandEnvironmentVariables($config.CustomDirs[$i])
                }
            }
            return $config
        } catch {
            Write-Color "  配置文件读取失败，将使用默认配置" -Color "DarkYellow"
        }
    }
    return $null
}

function Save-CustomConfig {
    param(
        [array]$CustomDirs,
        [array]$ExcludedDirs
    )
    
    $configPath = $Config.ConfigFile
    $configDir = Split-Path -Parent $configPath
    if (-not (Test-Path $configDir)) {
        New-Item -ItemType Directory -Path $configDir -Force | Out-Null
    }
    
    $config = @{
        CustomDirs  = $CustomDirs
        ExcludedDirs = $ExcludedDirs
        LastClean   = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
    }
    
    $config | ConvertTo-Json -Depth 3 | Set-Content -Path $configPath -Encoding UTF8
    Write-Color "  配置已保存" -Color "DarkGray"
}

# ============================================================
# 显示菜单和结果
# ============================================================

# 显示缓存列表
function Show-CacheList {
    param(
        [array]$Items,
        [string]$Title,
        [bool]$ShowIndex = $true
    )
    
    if ($Items.Count -eq 0) {
        Write-Color "  (无)" -Color "DarkGray"
        return
    }
    
    Write-Header -Title $Title
    
    # 按风险等级和大小排序
    $sorted = $Items | Sort-Object @{E={
        if ($_.Risk -eq "Safe") { 0 }
        elseif ($_.Risk -eq "Caution") { 1 }
        else { 2 }
    }}, @{E={$_.Size}} -Descending
    
    $index = 0
    $totalSize = 0
    
    foreach ($item in $sorted) {
        $index++
        $totalSize += $item.Size
        
        $riskColor = switch ($item.Risk) {
            "Safe"    { "Green" }
            "Caution" { "Yellow" }
            "Review"  { "Red" }
            default   { "White" }
        }
        
        $riskIcon = switch ($item.Risk) {
            "Safe"    { "[安全]" }
            "Caution" { "[注意]" }
            "Review"  { "[确认]" }
            default   { "[未知]" }
        }
        
        # 访问时间
        $accessStr = ""
        if ($item.LastAccess) {
            $daysOld = [math]::Round(((Get-Date) - $item.LastAccess).TotalDays, 0)
            if ($daysOld -gt 0) {
                $accessStr = " ($daysOld 天前访问)"
            }
        }
        
        $prefix = if ($ShowIndex) { "{0,2}. " -f $index } else { "    " }
        
        Write-Host ("$prefix[$($item.Category)] $riskIcon ") -NoNewline
        Write-Host ("$($item.ShortPath)") -NoNewline -ForegroundColor White
        Write-Host ("  $($item.SizeStr)") -NoNewline -ForegroundColor $riskColor
        Write-Host ("  $($item.FileCount) 文件$accessStr") -ForegroundColor DarkGray
        Write-Host ("     -> $($item.Desc)") -ForegroundColor DarkGray
    }
    
    Write-Color "`n  总计: $(Format-FileSize -Bytes $totalSize) - $($Items.Count) 项" -Color "Cyan"
}

# 交互选择清理
function Select-CleanItems {
    param(
        [array]$Items,
        [string]$Prompt = "选择要清理的项（输入编号，逗号分隔，如 1,3,5-8，或 all）"
    )
    
    if ($Items.Count -eq 0) {
        Write-Color "  没有可清理的项目。" -Color "Yellow"
        return @()
    }
    
    # 按风险等级和大小排序
    $sorted = $Items | Sort-Object @{E={
        if ($_.Risk -eq "Safe") { 0 }
        elseif ($_.Risk -eq "Caution") { 1 }
        else { 2 }
    }}, @{E={$_.Size}} -Descending
    
    Show-CacheList -Items $sorted -Title "缓存列表"
    
    Write-Host ""
    Write-Color "  操作说明:" -Color "DarkGray"
    Write-Color "    - 输入编号: 1,3,5 或 1-5 或 1,3,5-8" -Color "DarkGray"
    Write-Color "    - 输入 'all' 选择全部" -Color "DarkGray"
    Write-Color "    - 输入 'safe' 仅选择安全项" -Color "DarkGray"
    Write-Color "    - 输入 'q' 或回车 返回" -Color "DarkGray"
    Write-Host ""
    
    $input = Read-Host "  $Prompt"
    
    if ([string]::IsNullOrWhiteSpace($input) -or $input -eq 'q') {
        return @()
    }
    
    if ($input -eq 'all') {
        return $sorted
    }
    
    if ($input -eq 'safe') {
        return ($sorted | Where-Object { $_.Risk -eq "Safe" })
    }
    
    # 解析编号
    $selectedIndices = @()
    $parts = $input -split ','
    foreach ($part in $parts) {
        $part = $part.Trim()
        if ($part -match '^(\d+)-(\d+)$') {
            $start = [int]$matches[1]
            $end = [int]$matches[2]
            $selectedIndices += $start..$end
        } elseif ($part -match '^\d+$') {
            $selectedIndices += [int]$part
        }
    }
    
    $selected = @()
    foreach ($idx in $selectedIndices | Sort-Object -Unique) {
        if ($idx -ge 1 -and $idx -le $sorted.Count) {
            $selected += $sorted[$idx - 1]
        }
    }
    
    return $selected
}

# ============================================================
# 管理自定义目录
# ============================================================
function Manage-CustomDirs {
    $customConfig = Load-CustomConfig
    $customDirs = if ($customConfig -and $customConfig.CustomDirs) { @($customConfig.CustomDirs) } else { @() }
    $excludedDirs = if ($customConfig -and $customConfig.ExcludedDirs) { @($customConfig.ExcludedDirs) } else { @() }
    
    do {
        Clear-Host
        Write-Header -Title "自定义目录管理"
        
        Write-Color "  当前自定义目录:" -Color "Cyan"
        if ($customDirs.Count -eq 0) {
            Write-Color "    (暂无自定义目录)" -Color "DarkGray"
        } else {
            for ($i = 0; $i -lt $customDirs.Count; $i++) {
                if (Test-Path $customDirs[$i]) {
                    $size = Get-DirectorySize -Path $customDirs[$i]
                    Write-Host ("    {0,2}. {1}  [{2}]" -f ($i + 1), $customDirs[$i], (Format-FileSize -Bytes $size)) -ForegroundColor White
                } else {
                    Write-Host ("    {0,2}. {1}  [目录不存在]" -f ($i + 1), $customDirs[$i]) -ForegroundColor DarkYellow
                }
            }
        }
        
        Write-Host ""
        Write-Color "  操作:" -Color "Yellow"
        Write-Color "    1. 添加自定义目录" -Color "White"
        Write-Color "    2. 删除自定义目录" -Color "White"
        Write-Color "    3. 添加排除目录（不扫描）" -Color "White"
        if ($customDirs.Count -gt 0) { Write-Color "    4. 扫描并清理自定义目录" -Color "White" }
        Write-Color "    q. 返回主菜单" -Color "White"
        Write-Host ""
        
        $choice = Read-Host "  请选择"
        
        switch ($choice) {
            '1' {
                $newDir = Read-Host "  输入要添加的目录路径"
                if (-not [string]::IsNullOrWhiteSpace($newDir)) {
                    # 展开环境变量
                    $newDir = [Environment]::ExpandEnvironmentVariables($newDir)
                    if ($customDirs -notcontains $newDir) {
                        $customDirs += $newDir
                        Save-CustomConfig -CustomDirs $customDirs -ExcludedDirs $excludedDirs
                        Write-Color "  已添加: $newDir" -Color "Green"
                    } else {
                        Write-Color "  该目录已存在" -Color "Yellow"
                    }
                }
                Wait-KeyPress
            }
            '2' {
                if ($customDirs.Count -eq 0) { Write-Color "  没有可删除的目录" -Color "Yellow"; Wait-KeyPress; continue }
                $delIdx = Read-Host "  输入要删除的编号"
                $idx = 0
                $parsed = [int]::TryParse($delIdx, [ref]$idx)
                if (-not $parsed) { $idx = 0 }
                if ($idx -ge 1 -and $idx -le $customDirs.Count) {
                    $removed = $customDirs[$idx - 1]
                    $customDirs = @($customDirs | Where-Object { $_ -ne $removed })
                    Save-CustomConfig -CustomDirs $customDirs -ExcludedDirs $excludedDirs
                    Write-Color "  已删除: $removed" -Color "Green"
                }
                Wait-KeyPress
            }
            '3' {
                $newExclude = Read-Host "  输入要排除的目录名（如 node_modules, .git）"
                if (-not [string]::IsNullOrWhiteSpace($newExclude)) {
                    if ($excludedDirs -notcontains $newExclude) {
                        $excludedDirs += $newExclude
                        Save-CustomConfig -CustomDirs $customDirs -ExcludedDirs $excludedDirs
                        Write-Color "  已添加排除: $newExclude" -Color "Green"
                    }
                }
                Wait-KeyPress
            }
            '4' {
                if ($customDirs.Count -gt 0) {
                    $customItems = @()
                    foreach ($dir in $customDirs) {
                        if (Test-Path $dir) {
                            $size = Get-DirectorySize -Path $dir
                            $shortPath = $dir -replace [regex]::Escape($Config.UserHome + '\'), ''
                            $customItems += [PSCustomObject]@{
                                Path       = $dir
                                ShortPath  = $shortPath
                                Category   = "自定义"
                                Risk       = "Caution"
                                Desc       = "用户自定义目录"
                                Size       = $size
                                SizeStr    = Format-FileSize -Bytes $size
                                FileCount  = Get-DirectoryFileCount -Path $dir
                                LastAccess = Get-DirectoryLastAccess -Path $dir
                            }
                        }
                    }
                    if ($customItems.Count -gt 0) {
                        $selected = Select-CleanItems -Items $customItems
                        if ($selected.Count -gt 0) {
                            Write-Color "`n  确认清理以上 $($selected.Count) 项？(y/n)" -Color "Red" -NoNewline
                            $confirm = Read-Host
                            if ($confirm -eq 'y') {
                                Invoke-CleanCache -Items $selected
                            }
                        }
                    }
                }
                Wait-KeyPress
            }
        }
    } while ($choice -ne 'q')
}

# ============================================================
# 主菜单
# ============================================================
function Show-MainMenu {
    param([bool]$FirstRun = $false)
    
    do {
        Clear-Host
        
        # ===== 横幅（自适应宽度） =====
        $bw = Get-ConsoleWidth
        $bwInner = $bw - 2  # 去掉左右边框占位
        $bannerLine1 = "智能缓存清理工具 v2.0"
        $bannerLine2 = "Smart Cache Cleaner for Windows"
        Write-Color ("╔" + "═" * $bwInner + "╗") -Color "Cyan"
        Write-Color ("║" + (Center-Text -Text $bannerLine1 -Width $bwInner) + "║") -Color "Yellow"
        Write-Color ("║" + (Center-Text -Text $bannerLine2 -Width $bwInner) + "║") -Color "Cyan"
        Write-Color ("╚" + "═" * $bwInner + "╝") -Color "Cyan"
        Write-Host ""
        
        # ===== 系统信息 =====
        Write-Color "  [系统信息]" -Color "DarkGray"
        $osInfo = Get-WmiObject Win32_OperatingSystem -ErrorAction SilentlyContinue
        $osName = if ($osInfo) { $osInfo.Caption } else { "Windows" }
        Write-Color "  操作系统: $osName" -Color "DarkGray"
        $freeSpace = (Get-PSDrive C -ErrorAction SilentlyContinue).Free
        Write-Color "  C盘剩余: $(Format-FileSize -Bytes $freeSpace)" -Color "DarkGray"
        Write-Color "  目标目录: $($Config.UserHome)" -Color "DarkGray"
        Write-Host ""
        
        # ===== 功能菜单（自适应宽度） =====
        $menuWidth = $bw - 4  # 菜单框宽度（保留缩进）
        $menuInner = $menuWidth - 2
        Write-Color ("  ╔" + "═" * $menuInner + "╗") -Color "Magenta"
        Write-Color ("  ║" + (Center-Text -Text "主要功能 (AI缓存优先)" -Width $menuInner) + "║") -Color "Magenta"
        Write-Color ("  ╚" + "═" * $menuInner + "╝") -Color "Magenta"
        Write-Host ""
        Write-Color "    ┌───── AI 缓存清理（重点） ─────" -Color "White"
        Write-Color "    │ 1. 🔥 深度扫描 AI 缓存（推荐）" -Color "Magenta" -BackColor "Black"
        Write-Color "    │ 2. 一键扫描所有缓存" -Color "White"
        Write-Host ""
        Write-Color "    ├───── 分类清理 ─────" -Color "White"
        Write-Color "    │ 3. 清理 AI 缓存" -Color "White"
        Write-Color "    │ 4. 清理浏览器缓存" -Color "White"
        Write-Color "    │ 5. 清理系统临时文件" -Color "White"
        Write-Color "    │ 6. 清理开发工具缓存" -Color "White"
        Write-Color "    │ 7. 快速清理（仅安全项）" -Color "Green"
        Write-Host ""
        Write-Color "    ├───── 高级功能 ─────" -Color "White"
        Write-Color "    │ 8. 管理自定义目录" -Color "White"
        Write-Color "    │ 9. 扫描大文件(>50MB)" -Color "White"
        Write-Color "    │ 0. 查看历史清理记录" -Color "White"
        Write-Host ""
        Write-Color "    └─────" -Color "White"
        Write-Color "    q. 退出" -Color "White"
        Write-Host ""
        
        $choice = Read-Host "  请选择 [0-9]"
        
        switch ($choice) {
            '1' { Invoke-DeepAIClean }
            '2' { Invoke-SmartScan -Filter "all" }
            '3' { Invoke-SmartScan -Filter "AI缓存" }
            '4' { Invoke-SmartScan -Filter "浏览器缓存" }
            '5' { Invoke-SmartScan -Filter "系统临时文件" }
            '6' { Invoke-SmartScan -Filter "开发工具缓存" }
            '7' { Invoke-QuickClean }
            '8' { Manage-CustomDirs }
            '9' { Invoke-LargeFileScan }
            '0' { Show-CleanHistory }
            'q' { 
                Write-Color "`n  感谢使用，再见！" -Color "Cyan"
                return
            }
        }
        
        if ($choice -ne 'q' -and $choice -ne '') {
            Wait-KeyPress
        }
    } while ($choice -ne 'q')
}

# ============================================================
# 智能扫描（按分类筛选）
# ============================================================
function Invoke-SmartScan {
    param([string]$Filter = "all")
    
    Clear-Host
    Write-Header -Title "智能扫描中..."
    
    # 1. 扫描已知缓存
    $knownCaches = Find-KnownCaches
    
    # 2. 智能发现
    $discovered = Auto-DiscoverCaches
    
    # 3. 合并
    $allItems = $knownCaches + $discovered
    
    # 4. 按分类筛选
    if ($Filter -ne "all") {
        $allItems = $allItems | Where-Object { $_.Category -eq $Filter -or $_.Category -eq "自动发现" }
    }
    
    # 5. 去重（按路径）
    $seen = @{}
    $uniqueItems = @()
    foreach ($item in $allItems) {
        if (-not $seen.ContainsKey($item.Path)) {
            $seen[$item.Path] = $true
            $uniqueItems += $item
        }
    }
    
    Clear-Host
    Write-Header -Title "扫描结果"
    Write-Color "  总扫描项: $($uniqueItems.Count) 个缓存目录" -Color "Cyan"
    $totalSize = ($uniqueItems | Measure-Object -Property Size -Sum).Sum
    Write-Color "  总占用空间: $(Format-FileSize -Bytes $totalSize)" -Color "Cyan"
    Write-Host ""
    
    # 按分类统计
    $groups = $uniqueItems | Group-Object -Property Category
    foreach ($group in $groups) {
        $groupSize = ($group.Group | Measure-Object -Property Size -Sum).Sum
        Write-Color ("  [{0}] {1} 项, 占用 {2}" -f $group.Name, $group.Count, (Format-FileSize -Bytes $groupSize)) -Color "DarkGray"
    }
    Write-Host ""
    
    if ($uniqueItems.Count -eq 0) {
        Write-Color "  没有发现可清理的缓存。" -Color "Yellow"
        return
    }
    
    $selected = Select-CleanItems -Items $uniqueItems
    
    if ($selected.Count -gt 0) {
        Write-Host ""
        Write-Color ("  ⚠ 确认清理以上 $($selected.Count) 项 (共 $(Format-FileSize -Bytes ($selected | Measure-Object -Property Size -Sum).Sum))？") -Color "Red"
        $confirm = Read-Host "  输入 y 确认清理，输入 n 取消"
        
        if ($confirm -eq 'y') {
            $freed = Invoke-CleanCache -Items $selected
            
            # 保存清理记录
            $historyPath = "$PSScriptRoot\config\clean_history.json"
            $historyDir = Split-Path -Parent $historyPath
            if (-not (Test-Path $historyDir)) { New-Item -ItemType Directory -Path $historyDir -Force | Out-Null }
            
            $record = [PSCustomObject]@{
                Time = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
                Items = $selected.Count
                Freed = $freed
                FreedStr = Format-FileSize -Bytes $freed
                Category = $Filter
            }
            
            $history = @()
            if (Test-Path $historyPath) {
                try { $history = Get-Content -Path $historyPath -Raw -Encoding UTF8 | ConvertFrom-Json } catch {}
            }
            $history = @($history) + $record
            $history | ConvertTo-Json -Depth 3 | Set-Content -Path $historyPath -Encoding UTF8
        }
    }
}

# ============================================================
# 深度 AI 缓存清理（重点功能）
# ============================================================
function Invoke-DeepAIClean {
    Clear-Host
    Write-Header -Title "深度 AI 缓存扫描"
    Write-Host ""
    Write-Color "  正在全面扫描 AI 相关缓存，请稍候..." -ForegroundColor Cyan
    
    # 1. 已知 AI 缓存
    $aiCaches = Find-KnownCaches | Where-Object { $_.Category -eq "AI缓存" }
    
    # 2. 智能发现 AI 相关目录
    $discovered = Auto-DiscoverCaches | Where-Object { 
        $_.ShortPath -match 'cursor|trae|code|ai|AI|intellicode|copilot|vscode|vs code|chat|gpt|claude|ollama|hugging|torch|tensorflow|pytorch|openai|anthropic|machine.?learn|model'
    }
    
    # 3. 合并去重
    $seen = @{}
    $allItems = @()
    foreach ($item in ($aiCaches + $discovered)) {
        if (-not $seen.ContainsKey($item.Path)) {
            $seen[$item.Path] = $true
            $allItems += $item
        }
    }
    
    # 4. 额外扫描 AI 缓存特征文件
    Write-Host ""
    Write-Color "  正在深度扫描 AI 缓存文件..." -ForegroundColor DarkGray
    $aiCachePatterns = @(
        "*.aicache", "*.aichat", "*.aicontext", "*.completions",
        "*.suggestions", "*.copilot", "*.chatlog",
        "*.conversation", "*.conversations",
        "*.intellisense", "*.codeinsights",
        "*.embeddings", "*.vector*",
        "*.lspcache", "*.languageModel"
    )
    
    $extraFiles = @()
    $searchRoots = @(
        "$($Config.UserHome)\AppData\Roaming",
        "$($Config.UserHome)\AppData\Local",
        "$($Config.UserHome)"
    )
    
    foreach ($root in $searchRoots) {
        if (-not (Test-Path $root)) { continue }
        foreach ($pattern in $aiCachePatterns) {
            try {
                $files = Get-ChildItem -Path $root -Filter $pattern -Recurse -File -ErrorAction SilentlyContinue -Force -Depth 4
                foreach ($file in $files) {
                    $size = $file.Length
                    if ($size -gt 1024) {  # 忽略小于1KB的文件
                        $extraFiles += [PSCustomObject]@{
                            Path       = $file.FullName
                            ShortPath  = $file.FullName.Substring($Config.UserHome.Length).TrimStart('\')
                            Category   = "AI缓存(深度)"
                            Risk       = "Safe"
                            Desc       = "AI 缓存文件: $($file.Extension)"
                            Size       = $size
                            SizeStr    = Format-FileSize -Bytes $size
                            FileCount  = 1
                            LastAccess = $file.LastAccessTime
                        }
                    }
                }
            } catch {}
        }
    }
    
    # 合并额外文件
    $allItems += $extraFiles
    
    # 按大小排序
    $allItems = $allItems | Sort-Object -Property Size -Descending
    
    Clear-Host
    Write-Header -Title "深度 AI 缓存扫描结果"
    
    if ($allItems.Count -eq 0) {
        Write-Color "  没有发现 AI 缓存。" -Color "Yellow"
        return
    }
    
    $totalSize = ($allItems | Measure-Object -Property Size -Sum).Sum
    
    # 按子分类统计
    $aiGroups = $allItems | Group-Object -Property { 
        if ($_.ShortPath -match 'cursor|\.cursor') { "Cursor AI" }
        elseif ($_.ShortPath -match 'trae|Trae') { "Trae AI" }
        elseif ($_.ShortPath -match 'code|Code|vscode|VS Code') { "VS Code" }
        elseif ($_.ShortPath -match 'copilot|Copilot') { "GitHub Copilot" }
        elseif ($_.ShortPath -match 'ollama|hugging|torch|model') { "AI 模型" }
        elseif ($_.Category -eq "AI缓存(深度)") { "AI 缓存文件" }
        else { "其他 AI 缓存" }
    }
    
    Write-Color "  共发现 $($allItems.Count) 项 AI 缓存，总占用 $(Format-FileSize -Bytes $totalSize)" -Color "Cyan"
    Write-Host ""
    
    foreach ($group in $aiGroups) {
        $groupSize = ($group.Group | Measure-Object -Property Size -Sum).Sum
        $count = ($group.Group | Measure-Object).Count
        Write-Color "    [{0}] {1} 项, 占用 {2}" -f $group.Name, $count, (Format-FileSize -Bytes $groupSize) -Color "Magenta"
    }
    Write-Host ""
    
    $selected = Select-CleanItems -Items $allItems -Prompt "选择要清理的 AI 缓存项"
    
    if ($selected.Count -gt 0) {
        $selSize = ($selected | Measure-Object -Property Size -Sum).Sum
        Write-Host ""
        Write-Color ("  ⚠ 确认清理 $($selected.Count) 项 AI 缓存 (共 $(Format-FileSize -Bytes $selSize))？") -Color "Red"
        Write-Color "  (清理后 AI 编辑器可能需要重新加载一些数据，不影响已保存的代码)" -Color "DarkGray"
        $confirm = Read-Host "  输入 y 确认清理"
        
        if ($confirm -eq 'y') {
            $freed = Invoke-CleanCache -Items $selected
            
            # 保存清理记录
            $historyPath = "$PSScriptRoot\config\clean_history.json"
            $historyDir = Split-Path -Parent $historyPath
            if (-not (Test-Path $historyDir)) { New-Item -ItemType Directory -Path $historyDir -Force | Out-Null }
            
            $record = [PSCustomObject]@{
                Time = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
                Items = $selected.Count
                Freed = $freed
                FreedStr = Format-FileSize -Bytes $freed
                Category = "AI深度清理"
            }
            
            $history = @()
            if (Test-Path $historyPath) {
                try { $history = Get-Content -Path $historyPath -Raw -Encoding UTF8 | ConvertFrom-Json } catch {}
            }
            $history = @($history) + $record
            $history | ConvertTo-Json -Depth 3 | Set-Content -Path $historyPath -Encoding UTF8
        }
    }
}

# ============================================================
# 快速清理（仅安全项）
# ============================================================
function Invoke-QuickClean {
    Clear-Host
    Write-Header -Title "快速清理（仅安全项）"
    Write-Host ""
    
    $knownCaches = Find-KnownCaches
    $safeItems = $knownCaches | Where-Object { $_.Risk -eq "Safe" -and $_.Size -gt 0 }
    
    if ($safeItems.Count -eq 0) {
        Write-Color "  没有发现安全可清理的缓存。" -Color "Yellow"
        return
    }
    
    $totalSize = ($safeItems | Measure-Object -Property Size -Sum).Sum
    Write-Color "  发现 $($safeItems.Count) 项安全缓存，共占用 $(Format-FileSize -Bytes $totalSize)" -Color "Cyan"
    Write-Host ""
    
    Show-CacheList -Items $safeItems -Title "安全项列表"
    
    Write-Host ""
    Write-Color "  是否清理以上所有项目？(y/n)" -Color "Red" -NoNewline
    $confirm = Read-Host
    
    if ($confirm -eq 'y') {
        $freed = Invoke-CleanCache -Items $safeItems
    }
}

# ============================================================
# 大文件扫描
# ============================================================
function Invoke-LargeFileScan {
    Clear-Host
    Write-Header -Title "大文件扫描 (>50MB)"
    
    $largeFiles = Find-LargeFiles
    
    if ($largeFiles.Count -eq 0) {
        Write-Color "  没有发现大文件。" -Color "Yellow"
        return
    }
    
    $totalSize = ($largeFiles | Measure-Object -Property Size -Sum).Sum
    Write-Color "  发现 $($largeFiles.Count) 个大文件，共占用 $(Format-FileSize -Bytes $totalSize)" -Color "Cyan"
    Write-Host ""
    
    $index = 0
    foreach ($file in $largeFiles) {
        $index++
        $color = if ($file.DaysOld -gt 30) { "DarkGray" } else { "White" }
        Write-Host ("{0,2}. {1} [{2}] {3} 天前" -f $index, $file.ShortPath, $file.SizeStr, $file.DaysOld) -ForegroundColor $color
    }
    
    Write-Host ""
    Write-Color "  操作说明: 大文件清理请使用主菜单功能 1-6，大文件已被包含在缓存目录扫描中。" -Color "DarkGray"
}

# ============================================================
# 查看清理历史
# ============================================================
function Show-CleanHistory {
    $historyPath = "$PSScriptRoot\config\clean_history.json"
    
    Clear-Host
    Write-Header -Title "清理历史记录"
    
    if (-not (Test-Path $historyPath)) {
        Write-Color "  暂无清理记录。" -Color "Yellow"
        return
    }
    
    try {
        $history = Get-Content -Path $historyPath -Raw -Encoding UTF8 | ConvertFrom-Json
        if ($history -isnot [array]) { $history = @($history) }
        
        $totalFreed = ($history | Measure-Object -Property Freed -Sum).Sum
        $totalItems = ($history | Measure-Object -Property Items -Sum).Sum
        
        Write-Color "  累计清理: $totalItems 项, 共释放 $(Format-FileSize -Bytes $totalFreed)" -Color "Cyan"
        Write-Host ""
        
        Write-Color "  {0,-20} {1,8} {2,12} {3}" -f "时间", "项数", "释放空间", "分类" -Color "DarkGray"
        Write-Separator
        
        foreach ($record in $history) {
            $cat = if ($record.Category -eq "all") { "全部" } else { $record.Category }
            Write-Color "  {0,-20} {1,8} {2,12} {3}" -f $record.Time, $record.Items, $record.FreedStr, $cat -Color "White"
        }
    } catch {
        Write-Color "  读取历史记录失败" -Color "Red"
    }
}

# ============================================================
# 程序入口
# ============================================================

# 设置初始控制台窗口尺寸（CMD + PowerShell 同步配置）
$initialWindowWidth = 96
$initialWindowHeight = 28
$initialBufferHeight = 9999

# 方式一：通过 mode.com 设置 CMD 控制台窗口（兼容 cmd 启动场景）
try {
    & mode.com con: cols=$initialWindowWidth lines=$initialWindowHeight 2>$null
} catch { }

# 方式二：通过 PowerShell RawUI 设置窗口（兼容 PowerShell 直接启动场景）
try {
    $rawUI = $Host.UI.RawUI
    if ($rawUI) {
        $bufferSize = $rawUI.BufferSize
        if ($bufferSize.Width -lt $initialWindowWidth) {
            $rawUI.SetBufferSize($initialWindowWidth, [math]::Max($bufferSize.Height, $initialBufferHeight))
        }
        $rawUI.WindowSize = New-Object System.Management.Automation.Host.Size(
            [math]::Min($initialWindowWidth, $rawUI.BufferSize.Width),
            [math]::Min($initialWindowHeight, $rawUI.BufferSize.Height)
        )
        # 尝试设置窗口位置居中
        try {
            $rawUI.WindowPosition = New-Object System.Management.Automation.Host.Coordinates(
                [math]::Max(0, [int](($rawUI.BufferSize.Width - $rawUI.WindowSize.Width) / 2)),
                0
            )
        } catch { }
    }
} catch { }

# 方式三：通过 Win32 API 调整父窗口大小（最精确，覆盖 cmd/powershell/terminal）
try {
    Add-Type @"
    using System;
    using System.Runtime.InteropServices;
    public class ConsoleWin {
        [DllImport("kernel32.dll")] public static extern IntPtr GetConsoleWindow();
        [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hWnd, out RECT rect);
        [DllImport("user32.dll")] public static extern bool MoveWindow(IntPtr hWnd, int x, int y, int w, int h, bool repaint);
        [StructLayout(LayoutKind.Sequential)]
        public struct RECT { public int L,T,R,B; }
    }
"@ -ErrorAction SilentlyContinue

    $hwnd = [ConsoleWin]::GetConsoleWindow()
    if ($hwnd -ne [IntPtr]::Zero) {
        $rect = New-Object ConsoleWin+RECT
        [ConsoleWin]::GetWindowRect($hwnd, [ref]$rect) | Out-Null
        $charW = 8
        $charH = 16
        $extraW = 16
        $extraH = 52
        $targetW = $initialWindowWidth * $charW + $extraW
        $targetH = $initialWindowHeight * $charH + $extraH
        [ConsoleWin]::MoveWindow($hwnd, $rect.L, $rect.T, $targetW, $targetH, $true) | Out-Null
    }
} catch { }

# 检查管理员权限（非必须，但清理某些系统缓存需要）
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Color "  ⚠ 当前未以管理员身份运行" -Color "DarkYellow"
    Write-Color "  部分系统缓存可能需要管理员权限才能清理" -Color "DarkYellow"
    Write-Color "  建议右键脚本选择「以管理员身份运行」" -Color "DarkYellow"
    Write-Host ""
    Start-Sleep -Seconds 2
}

# 启动主菜单
Show-MainMenu