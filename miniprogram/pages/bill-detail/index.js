const { fetchBills } = require("../../services/bills");
const { createOrder, notifyMockSuccess, recordSubscribe } = require("../../services/pay");

Page({
  data: {
    billId: "",
    bill: null,
    payAmountYuan: "",
    paying: false,
    subscribing: false,
  },

  async onLoad(query) {
    this.setData({ billId: query.id || "" });
    await this.loadBill();
  },

  async loadBill() {
    const app = getApp();
    const token = app.globalData.token;
    if (!token) {
      wx.reLaunch({ url: "/pages/login-bind/index" });
      return;
    }

    wx.showLoading({ title: "加载中" });
    try {
      const data = await fetchBills(token);
      const hit = (data.records || []).find((r) => r.id === this.data.billId);
      if (!hit) {
        throw new Error("账单不存在");
      }
      const totalAmount = Number(hit.totalAmount || 0);
      const amountPaid = Number(hit.amountPaid || 0);
      const unpaid = Math.max(totalAmount - amountPaid, 0);
      const rentCycleMultiplier = { '月度': 1, '季度': 3, '半年': 6, '年度': 12 }[hit.rentCycle] || 1;
      const hasPrev = !!hit.hasPrev;
      this.setData({
        bill: {
          ...hit,
          totalAmount: totalAmount.toFixed(2),
          amountPaid: amountPaid.toFixed(2),
          unpaid: unpaid.toFixed(2),
          rentAmountDisplay: (Number(hit.rentAmount || 0) * rentCycleMultiplier).toFixed(2),
          waterBillDisplay: Number(hit.waterBill || 0).toFixed(2),
          electricityBillDisplay: Number(hit.electricityBill || 0).toFixed(2),
          waterReading: Number(hit.waterReading || 0).toFixed(1),
          electricityReading: Number(hit.electricityReading || 0).toFixed(1),
          prevWaterReading: Number(hit.prevWaterReading || 0).toFixed(1),
          prevElectricityReading: Number(hit.prevElectricityReading || 0).toFixed(1),
          waterUsage: Math.max(Number(hit.waterUsage || 0), 0).toFixed(1),
          electricityUsage: Math.max(Number(hit.electricityUsage || 0), 0).toFixed(1),
          hasPrev,
        },
      });
    } catch (error) {
      wx.showToast({ title: error.message || "加载失败", icon: "none" });
    } finally {
      wx.hideLoading();
    }
  },

  onPayAmountInput(e) {
    this.setData({ payAmountYuan: e.detail.value });
  },

  async onPay() {
    if (!this.data.bill) return;
    const app = getApp();
    const token = app.globalData.token;
    const unpaid = Number(this.data.bill.unpaid || 0);
    if (unpaid <= 0) {
      wx.showToast({ title: "该账单已结清", icon: "none" });
      return;
    }

    let amountFen = 0;
    if (this.data.payAmountYuan.trim()) {
      amountFen = Math.floor(Number(this.data.payAmountYuan) * 100);
      if (!Number.isFinite(amountFen) || amountFen <= 0) {
        wx.showToast({ title: "金额格式不正确", icon: "none" });
        return;
      }
    }

    this.setData({ paying: true });
    wx.showLoading({ title: "创建订单" });
    try {
      const order = await createOrder(token, this.data.bill.id, amountFen);
      await new Promise((resolve, reject) => {
        wx.requestPayment({
          ...order.payParams,
          success: resolve,
          fail: reject,
        });
      });

      // mock_mode 下主动通知后端入账，真实模式应由微信异步回调触发。
      await notifyMockSuccess(order.outTradeNo);
      wx.redirectTo({ url: `/pages/pay-result/index?outTradeNo=${order.outTradeNo}` });
    } catch (error) {
      wx.showToast({ title: error.message || "支付失败", icon: "none" });
    } finally {
      wx.hideLoading();
      this.setData({ paying: false });
    }
  },

  async onSubscribe() {
    const app = getApp();
    const token = app.globalData.token;
    this.setData({ subscribing: true });
    try {
      await new Promise((resolve, reject) => {
        wx.requestSubscribeMessage({
          tmplIds: ["REPLACE_WITH_TEMPLATE_ID"],
          success: resolve,
          fail: reject,
        });
      });
      await recordSubscribe(token, "REPLACE_WITH_TEMPLATE_ID");
      wx.showToast({ title: "已记录订阅授权", icon: "success" });
    } catch (error) {
      wx.showToast({ title: error.message || "订阅失败", icon: "none" });
    } finally {
      this.setData({ subscribing: false });
    }
  },
});
