// 系统信息 Demo 插件
// 演示 cc API 的用法：cc.log / cc.exec / cc.env / cc.platform / cc.render

// 1. 打日志（会写入主程序 CacheCleaner.log，也会展示在面板底部）
cc.log('插件启动，平台: ' + cc.platform());

// 2. 执行系统命令读取信息
let osInfo = '';
let memInfo = '';

if (cc.platform() === 'windows') {
  // Windows: 用 wmic / ver 读取
  const ver = cc.exec('ver', 5000);
  osInfo = ver.stdout.trim();
} else {
  const uname = cc.exec('uname -a', 5000);
  osInfo = uname.stdout.trim();
}

// 3. 读环境变量
const home = cc.home();

// 4. 渲染结果（HTML）
cc.render({
  title: '系统信息',
  html: `
    <div style="padding: 16px; line-height: 1.8;">
      <h3 style="margin:0 0 12px;">🖥️ 系统信息</h3>
      <table style="width:100%; border-collapse: collapse; font-size: 14px;">
        <tr><td style="padding:6px 0; color:#888;">平台</td><td>${cc.platform()}</td></tr>
        <tr><td style="padding:6px 0; color:#888;">系统版本</td><td>${osInfo}</td></tr>
        <tr><td style="padding:6px 0; color:#888;">用户目录</td><td>${home}</td></tr>
      </table>
    </div>
  `,
});

cc.log('系统信息读取完成');
