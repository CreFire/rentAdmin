const { queryOrder } = require("../../services/pay");

Page({
  data: {
    outTradeNo: "",
    tradeState: "未知",
    paidAmount: "",
  },

  onLoad(query) {
    this.setData({ outTradeNo: query.outTradeNo || "" });
  },

  onShow() {
    this.onRefresh();
  },

  async onRefresh() {
    const app = getApp();
    const token = app.globalData.token;
    if (!token || !this.data.outTradeNo) return;

    wx.showLoading({ title: "查询中" });
    try {
      const order = await queryOrder(token, this.data.outTradeNo);
      this.setData({
        tradeState: order.tradeState || "未知",
        paidAmount: order.amountFen ? (Number(order.amountFen) / 100).toFixed(2) : "",
      });
    } catch (error) {
      wx.showToast({ title: error.message || "查询失败", icon: "none" });
    } finally {
      wx.hideLoading();
    }
  },

  goBills() {
    wx.redirectTo({ url: "/pages/bills/index" });
  },
});
