const { request } = require("./http");

/**
 * 调用 wx.login 拿 code，再向后端换取业务 token / openid。
 * 失败时抛出语义化 Error，便于页面用 wx.showToast 直接展示。
 */
async function loginByWeChatCode() {
  const loginResult = await new Promise((resolve, reject) => {
    wx.login({
      success: resolve,
      fail: (err) => {
        const msg = (err && err.errMsg) || "wx.login 调用失败";
        // 常见原因：开发者工具未配置 appid、或基础库异常
        reject(new Error(`微信登录失败：${msg}`));
      },
    });
  });

  if (!loginResult || !loginResult.code) {
    throw new Error("微信未返回 code，请检查小程序 appid 是否已在 project.config.json 中配置");
  }

  return request({
    url: "/api/mp/login",
    method: "POST",
    data: { code: loginResult.code },
  });
}

async function bindRoom(token, roomNumber, tenantName) {
  return request({
    url: "/api/mp/bind",
    method: "POST",
    token,
    data: { roomNumber, tenantName },
  });
}

module.exports = { loginByWeChatCode, bindRoom };

