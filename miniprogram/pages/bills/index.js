const { fetchBills } = require("../../services/bills");

function withViewData(record) {
  const totalAmount = Number(record.totalAmount || 0).toFixed(2);
  const amountPaid = Number(record.amountPaid || 0).toFixed(2);
  const unpaid = Math.max(Number(totalAmount) - Number(amountPaid), 0).toFixed(2);
  let statusClass = "status-unpaid";
  if (record.status === "已缴") statusClass = "status-paid";
  if (record.status === "部分缴纳") statusClass = "status-partial";
  return { ...record, totalAmount, amountPaid, unpaid, statusClass };
}

Page({
  data: {
    roomNumber: "",
    records: [],
  },

  onShow() {
    this.loadBills();
  },

  async onRefresh() {
    await this.loadBills();
  },

  async loadBills() {
    const app = getApp();
    if (!app.globalData.token) {
      wx.reLaunch({ url: "/pages/login-bind/index" });
      return;
    }

    wx.showLoading({ title: "加载中" });
    try {
      const data = await fetchBills(app.globalData.token);
      const records = (data.records || []).map(withViewData);
      this.setData({
        roomNumber: data.roomNumber || app.globalData.roomNumber || "",
        records,
      });
    } catch (error) {
      wx.showToast({ title: error.message || "加载失败", icon: "none" });
    } finally {
      wx.hideLoading();
    }
  },

  openDetail(e) {
    const id = e.currentTarget.dataset.id;
    wx.navigateTo({ url: `/pages/bill-detail/index?id=${id}` });
  },
});
