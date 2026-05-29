const STORAGE_KEY = "rentadmin_auth";

App({
  globalData: {
    token: "",
    openid: "",
    roomNumber: "",
    expiresAt: 0,
  },

  onLaunch() {
    try {
      const cached = wx.getStorageSync(STORAGE_KEY);
      if (cached && typeof cached === "object") {
        if (cached.expiresAt && Number(cached.expiresAt) * 1000 < Date.now()) {
          // token 已过期，清掉缓存
          wx.removeStorageSync(STORAGE_KEY);
          return;
        }
        this.globalData.token = cached.token || "";
        this.globalData.openid = cached.openid || "";
        this.globalData.roomNumber = cached.roomNumber || "";
        this.globalData.expiresAt = Number(cached.expiresAt) || 0;
      }
    } catch (e) {
      // 读取缓存失败时忽略，按未登录处理
    }
  },

  /**
   * 持久化登录态。新值会与现有 globalData 合并后写入 storage。
   * @param {{token?: string, openid?: string, roomNumber?: string, expiresAt?: number}} patch
   */
  setAuth(patch) {
    Object.assign(this.globalData, patch || {});
    try {
      wx.setStorageSync(STORAGE_KEY, {
        token: this.globalData.token,
        openid: this.globalData.openid,
        roomNumber: this.globalData.roomNumber,
        expiresAt: this.globalData.expiresAt,
      });
    } catch (e) {
      // 持久化失败不阻塞业务
    }
  },

  clearAuth() {
    this.globalData.token = "";
    this.globalData.openid = "";
    this.globalData.roomNumber = "";
    this.globalData.expiresAt = 0;
    try {
      wx.removeStorageSync(STORAGE_KEY);
    } catch (e) {
      // 忽略
    }
  },
});
