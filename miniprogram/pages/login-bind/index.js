const { loginByWeChatCode, bindRoom } = require("../../services/auth");

Page({
  data: {
    // 步骤：login = 等待微信登录；bind = 已登录待绑定房号
    step: "login",
    roomNumber: "",
    tenantName: "",
    openid: "",
    loginLoading: false,
    bindLoading: false,
  },

  onLoad() {
    const app = getApp();
    const { token, roomNumber, openid } = app.globalData || {};
    if (token && roomNumber) {
      // 已登录且已绑定，直接进账单页
      wx.reLaunch({ url: "/pages/bills/index" });
      return;
    }
    if (token) {
      // 已登录未绑定，跳到第二步
      this.setData({ step: "bind", openid: openid || "" });
    }
  },

  onRoomInput(e) {
    this.setData({ roomNumber: e.detail.value });
  },

  onNameInput(e) {
    this.setData({ tenantName: e.detail.value });
  },

  /** 第一步：仅做微信登录，换取后端 token */
  async onLogin() {
    if (this.data.loginLoading) return;
    this.setData({ loginLoading: true });
    try {
      const loginData = await loginByWeChatCode();
      if (!loginData || !loginData.token) {
        throw new Error("登录响应缺少 token，请联系管理员");
      }
      const app = getApp();
      app.setAuth({
        token: loginData.token,
        openid: loginData.openid || "",
        expiresAt: Number(loginData.expiresAt) || 0,
      });

      this.setData({ step: "bind", openid: loginData.openid || "" });
      wx.showToast({ title: "登录成功", icon: "success" });
    } catch (error) {
      wx.showModal({
        title: "登录失败",
        content: error.message || "未知错误",
        showCancel: false,
      });
    } finally {
      this.setData({ loginLoading: false });
    }
  },

  /** 第二步：绑定房号（首次使用即"注册"） */
  async onBindRoom() {
    if (this.data.bindLoading) return;
    const roomNumber = this.data.roomNumber.trim();
    if (!roomNumber) {
      wx.showToast({ title: "请输入房号", icon: "none" });
      return;
    }

    const app = getApp();
    const token = app.globalData.token;
    if (!token) {
      // 兜底：万一 token 丢了，回到第一步
      this.setData({ step: "login" });
      wx.showToast({ title: "请先点击微信登录", icon: "none" });
      return;
    }

    this.setData({ bindLoading: true });
    try {
      await bindRoom(token, roomNumber, this.data.tenantName.trim());
      app.setAuth({ roomNumber });
      wx.showToast({ title: "绑定成功", icon: "success" });
      setTimeout(() => {
        wx.reLaunch({ url: "/pages/bills/index" });
      }, 600);
    } catch (error) {
      wx.showModal({
        title: "绑定失败",
        content: error.message || "未知错误",
        showCancel: false,
      });
    } finally {
      this.setData({ bindLoading: false });
    }
  },

  /** 切回第一步（重新登录），方便切换微信账号或排错 */
  onReLogin() {
    const app = getApp();
    app.clearAuth();
    this.setData({ step: "login", roomNumber: "", tenantName: "", openid: "" });
  },
});
