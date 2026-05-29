const { ENV } = require("../config/env");

function request({ url, method = "GET", data, token }) {
  return new Promise((resolve, reject) => {
    wx.request({
      url: `${ENV.baseURL}${url}`,
      method,
      timeout: ENV.requestTimeout,
      data,
      header: {
        "Content-Type": "application/json",
        Authorization: token ? `Bearer ${token}` : "",
      },
      success: (res) => {
        if (res.statusCode >= 200 && res.statusCode < 300) {
          resolve(res.data);
          return;
        }
        if (res.statusCode === 401) {
          // token 失效，清掉本地缓存，下一次进入会回到登录页
          try {
            const app = getApp();
            if (app && typeof app.clearAuth === "function") app.clearAuth();
          } catch (e) {
            // 忽略
          }
        }
        const message = (res.data && res.data.error) || `请求失败: ${res.statusCode}`;
        reject(new Error(message));
      },
      fail: (err) => {
        const errMsg = (err && err.errMsg) || "网络请求失败";
        // 真机调试时若仍配置 127.0.0.1 / localhost 会触发 fail；给出明确提示
        if (/url not in domain list|fail .*ssl|fail timeout|fail .*ENOTFOUND|fail$/i.test(errMsg)) {
          reject(new Error(`无法连接到后端（${ENV.baseURL}）：${errMsg}。请确认后端已启动，且开发者工具勾选「不校验合法域名」。真机调试需把 baseURL 改为局域网 IP。`));
          return;
        }
        reject(new Error(errMsg));
      },
    });
  });
}

module.exports = { request };
