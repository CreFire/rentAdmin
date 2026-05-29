/**
 * @license
 * SPDX-License-Identifier: Apache-2.0
*/

import React, { useState, useMemo, useEffect, useCallback } from 'react';
import ReactDOM from 'react-dom/client';
import { ExcelImportSummary, TenantRecord, PaymentStatus, PaymentCycle } from './types';
import { INITIAL_RECORDS } from './constants';
import { fetchAllTenants, createOrUpdateTenant, updateTenant, fetchTenantsByRoom, fetchIncomeSummary, importExcelTenants, deleteTenantByRoom, deleteTenantById } from './utils';

const WATER_UNIT_PRICE = 5.5;
const ELEC_UNIT_PRICE = 1.2;

const getRentCycleMultiplier = (cycle?: PaymentCycle): number => {
  switch (cycle) {
    case '月度':
      return 1;
    case '季度':
      return 3;
    case '半年':
      return 6;
    case '年度':
      return 12;
    default:
      return 1;
  }
};

function useTenants() {
  const [records, setRecords] = useState<TenantRecord[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const loadRecords = async () => {
      try {
        const data = await fetchAllTenants();
        setRecords(data);
      } catch (error) {
        console.error('Failed to load records:', error);
        setRecords([]);
      } finally {
        setLoading(false);
      }
    };

    loadRecords();

    // Set up polling to refresh data periodically
    const intervalId = setInterval(loadRecords, 30000); // Refresh every 30 seconds
    
    return () => clearInterval(intervalId);
  }, []);

  // Function to refresh records after changes
  const refreshRecords = useCallback(async () => {
    try {
      const data = await fetchAllTenants();
      setRecords(data);
    } catch (error) {
      console.error('Failed to refresh records:', error);
    }
  }, []);

  const updateRecordCallback = useCallback(async (id: string, data: Partial<TenantRecord>) => {
    await updateTenant(id, data as TenantRecord);
    await refreshRecords(); // Refresh after update
  }, [refreshRecords]);

  const addRecordCallback = useCallback(async (data: Partial<TenantRecord>) => {
    await createOrUpdateTenant(data as TenantRecord);
    await refreshRecords(); // Refresh after adding
  }, [refreshRecords]);

  return { records, loading, updateRecord: updateRecordCallback, addRecord: addRecordCallback, refreshRecords };
}

const SearchIcon = () => <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/></svg>;
const UserIcon = () => <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>;
const MeterIcon = () => <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M12 2v2"/><circle cx="12" cy="12" r="8"/><path d="M12 12V6"/></svg>;
const HistoryIcon = () => <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M12 8v4l3 3m6-3a9 9 0 1 1-18 0 9 9 0 0 1 18 0z"/></svg>;
const EditIcon = () => <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>;
const TrashIcon = () => <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M3 6h18"/><path d="M8 6V4h8v2"/><path d="M19 6l-1 14H6L5 6"/><path d="M10 11v6"/><path d="M14 11v6"/></svg>;
const PayIcon = () => <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><rect width="20" height="14" x="2" y="5" rx="2"/><line x1="2" y1="10" x2="22" y2="10"/></svg>;
const SparkleIcon = () => <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="m12 3-1.912 5.813a2 2 0 0 1-1.275 1.275L3 12l5.813 1.912a2 2 0 0 1 1.275 1.275L12 21l1.912-5.813a2 2 0 0 1 1.275-1.275L21 12l-5.813-1.912a2 2 0 0 1-1.275-1.275L12 3Z"/></svg>;
const ChartIcon = () => <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg>;

function SmartReminderModal({ record, onClose }: { record: TenantRecord, onClose: () => void }) {
  const [content, setContent] = useState('AI 正在为您起草提醒文案...');
  const [generating, setGenerating] = useState(true);

  useEffect(() => {
    async function generate() {
      try {
        // Mock AI generation for now
        const unpaid = record.totalAmount - record.amountPaid;
        setContent(`您好 ${record.name}，您房号${record.roomNumber}在${record.date}的租金及相关费用共计${unpaid.toFixed(2)}元尚未缴纳，请尽快处理，谢谢！`);
      } catch (e) { 
        setContent('生成失败，请检查 API 配置。'); 
      } finally { 
        setGenerating(false); 
      }
    }
    generate();
  }, [record]);

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content mini" onClick={e => e.stopPropagation()}>
        <div className="main-header"><h3>智能催款提示</h3></div>
        <div className={`ai-content-box ${generating ? 'loading' : ''}`}>{content}</div>
        <div style={{ marginTop: 16, display: 'flex', gap: 10 }}>
          <button className="btn-primary" style={{ flex: 1 }} onClick={() => { navigator.clipboard.writeText(content); alert('已复制'); }}>复制</button>
          <button className="btn-submit" style={{ flex: 1, marginTop: 0, background: 'var(--border-bright)' }} onClick={onClose}>关闭</button>
        </div>
      </div>
    </div>
  );
}

function IncomeStatsPanel() {
  const [incomeStats, setIncomeStats] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [dateFilter, setDateFilter] = useState('');

  useEffect(() => {
    const fetchStats = async () => {
      try {
        const stats = await fetchIncomeSummary(dateFilter);
        setIncomeStats(stats);
      } catch (error) {
        console.error('Failed to fetch income stats:', error);
      } finally {
        setLoading(false);
      }
    };

    fetchStats();
  }, [dateFilter]);

  if (loading) {
    return (
      <div className="view-fade-in">
        <header className="main-header">
          <div><h1>收入统计</h1><p className="subtitle">正在加载统计数据...</p></div>
        </header>
        <div className="stats-strip">
          <div className="stat-item"><span className="label">累计应收</span><span className="value">--</span></div>
          <div className="stat-item"><span className="label">累计实收</span><span className="value">--</span></div>
          <div className="stat-item"><span className="label">水电收入</span><span className="value">--</span></div>
          <div className="stat-item"><span className="label">代收余额</span><span className="value">--</span></div>
        </div>
      </div>
    );
  }

  return (
    <div className="view-fade-in">
      <header className="main-header">
        <div><h1>收入统计</h1><p className="subtitle">详细收入分析和汇总</p></div>
        <div className="action-group">
          <input 
            type="month" 
            value={dateFilter} 
            onChange={(e) => setDateFilter(e.target.value)}
            placeholder="选择月份" 
            style={{ padding: '8px', borderRadius: '4px', border: '1px solid var(--border)' }}
          />
        </div>
      </header>
      
      <div className="stats-strip">
        <div className="stat-item"><span className="label">累计应收</span><span className="value">¥{incomeStats?.totalReceivable?.toFixed(2) || 0}</span></div>
        <div className="stat-item"><span className="label">累计实收</span><span className="value">¥{incomeStats?.totalReceived?.toFixed(2) || 0}</span></div>
        <div className="stat-item"><span className="label">水电收入</span><span className="value">¥{incomeStats?.totalUtilityIncome?.toFixed(2) || 0}</span></div>
        <div className="stat-item"><span className="label">代收余额</span><span className="value" style={{ color: 'var(--warning)' }}>¥{incomeStats?.outstandingBalance?.toFixed(2) || 0}</span></div>
      </div>
      
      <div className="table-wrapper" style={{ marginTop: '24px' }}>
        <table className="admin-table">
          <thead>
            <tr>
              <th>统计项</th>
              <th>金额</th>
              <th>说明</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>累计应收</td>
              <td>¥{incomeStats?.totalReceivable?.toFixed(2) || 0}</td>
              <td>所有账单的应收费用总额</td>
            </tr>
            <tr>
              <td>累计实收</td>
              <td>¥{incomeStats?.totalReceived?.toFixed(2) || 0}</td>
              <td>实际收到的款项总额</td>
            </tr>
            <tr>
              <td>水电收入</td>
              <td>¥{incomeStats?.totalUtilityIncome?.toFixed(2) || 0}</td>
              <td>水费和电费的总收入</td>
            </tr>
            <tr>
              <td>代收余额</td>
              <td style={{ color: 'var(--warning)' }}>¥{incomeStats?.outstandingBalance?.toFixed(2) || 0}</td>
              <td>尚未收回的款项</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  );
}

function App() {
  const { records, updateRecord, addRecord, refreshRecords } = useTenants();
  const [activePage, setActivePage] = useState<'billing' | 'metering' | 'collection' | 'management' | 'history' | 'income'>('management');
  const [modalState, setModalState] = useState<{ type: 'edit' | 'pay' | 'meter' | 'ai' | null, record: TenantRecord | null }>({ type: null, record: null });
  const [editMode, setEditMode] = useState<'tenant' | 'history'>('tenant');
  const [searchQuery, setSearchQuery] = useState('');
  const [importingExcel, setImportingExcel] = useState(false);
  const [deletingTenant, setDeletingTenant] = useState(false);
  const [importSummary, setImportSummary] = useState<ExcelImportSummary | null>(null);
  const [importError, setImportError] = useState('');
  const [formData, setFormData] = useState({
    roomNumber: '', name: '', phone: '', idCard: '', checkInDate: '', deposit: '', rentAmount: '', waterReading: '', electricityReading: '', rentCycle: '月度' as PaymentCycle, utilityCycle: '月度' as PaymentCycle
  });
  const [payAmount, setPayAmount] = useState('');
  const [meterInput, setMeterInput] = useState({ water: '', electricity: '', date: new Date().toISOString().slice(0, 10) });
  const [historyRoomId, setHistoryRoomId] = useState<string | null>(null);

  const pendingCollection = useMemo(() => records.filter(r => r.status !== '已缴'), [records]);
  const uniqueTenants = useMemo(() => {
    const seen = new Set();
    return records.filter(r => {
      const duplicate = seen.has(r.roomNumber);
      seen.add(r.roomNumber);
      return !duplicate;
    });
  }, [records]);

  const stats = useMemo(() => {
    const total = records.reduce((acc, r) => acc + (r.totalAmount || 0), 0);
    const paid = records.reduce((acc, r) => acc + (r.amountPaid || 0), 0);
    return { total, unpaid: total - paid, pending: pendingCollection.length, count: uniqueTenants.length };
  }, [records, pendingCollection, uniqueTenants]);

  const roomHistory = useMemo(() => {
    if (!historyRoomId) return [];
    return records.filter(r => r.roomNumber === historyRoomId).sort((a, b) => b.date.localeCompare(a.date));
  }, [records, historyRoomId]);

  const getPrevReading = useCallback((roomNumber: string, date: string) => {
    const sorted = records
      .filter(r => r.roomNumber === roomNumber && r.date < date.slice(0, 7))
      .sort((a, b) => b.date.localeCompare(a.date));
    return sorted[0] || null;
  }, [records]);

  // Helper function to safely calculate amounts
  const calculateAmount = (value: number | undefined): number => {
    if (value === undefined || isNaN(value) || value === null) {
      return 0;
    }
    return Math.max(0, value);
  };

  const openEdit = (record: TenantRecord | null, mode: 'tenant' | 'history' = 'tenant') => {
    setEditMode(mode);
    if (record) {
      setFormData({
        roomNumber: record.roomNumber, name: record.name, phone: record.phone || '', idCard: record.idCard || '', checkInDate: record.checkInDate || '', deposit: record.deposit?.toString() || '', rentAmount: record.rentAmount.toString(),
        waterReading: record.waterReading?.toString() || '0',
        electricityReading: record.electricityReading?.toString() || '0',
        rentCycle: record.rentCycle || '月度',
        utilityCycle: record.utilityCycle || '月度'
      });
      setModalState({ type: 'edit', record });
    } else {
      setEditMode('tenant');
      setFormData({ roomNumber: '', name: '', phone: '', idCard: '', checkInDate: '', deposit: '', rentAmount: '', waterReading: '', electricityReading: '', rentCycle: '月度', utilityCycle: '月度' });
      setModalState({ type: 'edit', record: null });
    }
  };

  const handleFormSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const curWater = parseFloat(formData.waterReading) || 0;
    const curElec = parseFloat(formData.electricityReading) || 0;
    const dateStr = new Date().toISOString().slice(0, 7);
    const prev = getPrevReading(formData.roomNumber, dateStr);
    const waterBill = prev ? (curWater - prev.waterReading) * WATER_UNIT_PRICE : 0;
    const electricityBill = prev ? (curElec - prev.electricityReading) * ELEC_UNIT_PRICE : 0;
    const baseRent = parseFloat(formData.rentAmount) || 0;
    const rentMultiplier = getRentCycleMultiplier(formData.rentCycle);
    const rentAmountByCycle = baseRent * rentMultiplier;
    const total = rentAmountByCycle + waterBill + electricityBill;
    
    const data: Partial<TenantRecord> = {
      ...formData,
      rentAmount: baseRent,
      deposit: parseFloat(formData.deposit) || 0,
      waterReading: curWater, electricityReading: curElec,
      waterBill, electricityBill, totalAmount: total,
      amountPaid: modalState.record?.amountPaid || 0,
      date: dateStr, recordedAt: new Date().toISOString().slice(0, 10),
      status: (modalState.record?.amountPaid || 0) >= total ? '已缴' : ((modalState.record?.amountPaid || 0) > 0 ? '部分缴纳' : '待缴')
    };
    if (modalState.record) await updateRecord(modalState.record.id, data);
    else await addRecord(data);
    setModalState({ type: null, record: null });
  };

  const handleMeterSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!modalState.record) return;
    const curWater = parseFloat(meterInput.water) || 0;
    const curElec = parseFloat(meterInput.electricity) || 0;
    const month = meterInput.date.slice(0, 7);
    const prev = getPrevReading(modalState.record.roomNumber, month);
    const waterBill = prev ? (curWater - prev.waterReading) * WATER_UNIT_PRICE : 0;
    const electricityBill = prev ? (curElec - prev.electricityReading) * ELEC_UNIT_PRICE : 0;
    const rentMultiplier = getRentCycleMultiplier(modalState.record.rentCycle);
    const total = (modalState.record.rentAmount * rentMultiplier) + waterBill + electricityBill;

    const existing = records.find(r => r.roomNumber === modalState.record!.roomNumber && r.date === month);
    const data: Partial<TenantRecord> = {
      waterReading: curWater, electricityReading: curElec, waterBill, electricityBill, totalAmount: total,
      date: month, recordedAt: meterInput.date,
      status: (existing?.amountPaid || 0) >= total ? '已缴' : ((existing?.amountPaid || 0) > 0 ? '部分缴纳' : '待缴')
    };

    if (existing) await updateRecord(existing.id, data);
    else await addRecord({ ...modalState.record, ...data, id: undefined, amountPaid: 0 });
    setModalState({ type: null, record: null });
  };

  const handlePaySubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!modalState.record) return;
    const newPaid = modalState.record.amountPaid + (parseFloat(payAmount) || 0);
    const status = newPaid >= modalState.record.totalAmount ? '已缴' : (newPaid > 0 ? '部分缴纳' : '待缴');
    await updateRecord(modalState.record.id, { amountPaid: newPaid, status });
    setModalState({ type: null, record: null });
    setPayAmount('');
  };

  const handleExcelImport = useCallback(async () => {
    setImportingExcel(true);
    setImportError('');
    try {
      const summary = await importExcelTenants();
      setImportSummary(summary);
      await refreshRecords();
    } catch (error) {
      console.error('Failed to import excel:', error);
      setImportError(error instanceof Error ? error.message : '导入失败，请检查后端日志。');
    } finally {
      setImportingExcel(false);
    }
  }, [refreshRecords]);

  const handleDeleteTenant = useCallback(async () => {
    if (!modalState.record) return;

    const confirmed = window.confirm(`确认删除房号 ${modalState.record.roomNumber} 的全部租客记录吗？此操作不可恢复。`);
    if (!confirmed) return;

    setDeletingTenant(true);
    try {
      await deleteTenantByRoom(modalState.record.roomNumber);
      await refreshRecords();
      setModalState({ type: null, record: null });
      alert('租客已删除');
    } catch (error) {
      console.error('Failed to delete tenant:', error);
      alert(error instanceof Error ? error.message : '删除失败');
    } finally {
      setDeletingTenant(false);
    }
  }, [modalState.record, refreshRecords]);

  const handleDeleteHistoryRecord = useCallback(async (record: TenantRecord) => {
    const confirmed = window.confirm(`确认删除 ${record.roomNumber} 在 ${record.date} 的审计记录吗？此操作不可恢复。`);
    if (!confirmed) return;

    try {
      await deleteTenantById(record.id);
      await refreshRecords();
      if (modalState.record?.id === record.id) {
        setModalState({ type: null, record: null });
      }
      alert('审计记录已删除');
    } catch (error) {
      console.error('Failed to delete history record:', error);
      alert(error instanceof Error ? error.message : '删除失败');
    }
  }, [modalState.record, refreshRecords]);

  return (
    <div className="admin-layout">
      <aside className="admin-sidebar">
        <div className="sidebar-brand"><div className="brand-logo">R</div><span>RentAdmin</span></div>
        <nav className="sidebar-nav">
          <button className={activePage === 'billing' ? 'active' : ''} onClick={() => setActivePage('billing')}>
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><rect width="7" height="9" x="3" y="3" rx="1"/><rect width="7" height="5" x="14" y="3" rx="1"/><rect width="7" height="9" x="14" y="12" rx="1"/><rect width="7" height="5" x="3" y="16" rx="1"/></svg>
            账单中心
          </button>
          <button className={activePage === 'metering' ? 'active' : ''} onClick={() => setActivePage('metering')}>
             <MeterIcon /> 水电录入
          </button>
          <button className={activePage === 'collection' ? 'active' : ''} onClick={() => setActivePage('collection')}>
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"/></svg>
            催缴任务
          </button>
          <button className={activePage === 'management' ? 'active' : ''} onClick={() => setActivePage('management')}>
            <UserIcon /> 租户档案
          </button>
          <button className={activePage === 'income' ? 'active' : ''} onClick={() => setActivePage('income')}>
            <ChartIcon /> 收入统计
          </button>
        </nav>
      </aside>

      <main className="admin-main">
        <section className="stats-strip">
          <div className="stat-item"><span className="label">累计应收</span><span className="value">¥{stats.total.toFixed(0)}</span></div>
          <div className="stat-item"><span className="label">待收余额</span><span className="value" style={{ color: 'var(--warning)' }}>¥{stats.unpaid.toFixed(0)}</span></div>
          <div className="stat-item"><span className="label">催缴笔数</span><span className="value">{stats.pending}</span></div>
          <div className="stat-item"><span className="label">租户总数</span><span className="value">{stats.count}</span></div>
        </section>

        {activePage === 'billing' && (
          <div className="view-fade-in">
            <header className="main-header">
              <div><h1>账务结算</h1><p className="subtitle">管理所有房间的应收与实收汇总</p></div>
              <div className="action-group">
                <div className="search-box"><SearchIcon /><input type="text" placeholder="搜索房号..." value={searchQuery} onChange={e => setSearchQuery(e.target.value)} /></div>
                <button className="btn-secondary" onClick={handleExcelImport} disabled={importingExcel}>
                  {importingExcel ? '导入中...' : '导入 Excel'}
                </button>
                <button className="btn-primary" onClick={() => openEdit(null)}>新开账单</button>
              </div>
            </header>
            {importError && (
              <div className="import-summary import-summary-error">
                <strong>导入失败</strong>
                <span>{importError}</span>
              </div>
            )}
            {importSummary && (
              <div className="import-summary">
                <strong>导入完成</strong>
                <span>处理工作表 {importSummary.processedSheets} 个，新增 {importSummary.inserted} 条，更新 {importSummary.updated} 条，跳过 {importSummary.skipped} 个。</span>
                {importSummary.errors.length > 0 && (
                  <span>异常: {importSummary.errors.join(' | ')}</span>
                )}
              </div>
            )}
            <div className="table-wrapper">
              <table className="admin-table">
                <thead><tr><th>房号</th><th>姓名</th><th>当前账期</th><th>本期合计</th><th>已缴</th><th>状态</th><th>操作</th></tr></thead>
                <tbody>
                  {uniqueTenants.filter(r => r.roomNumber.includes(searchQuery)).map(r => {
                    const latest = records.filter(rec => rec.roomNumber === r.roomNumber).sort((a,b) => b.date.localeCompare(a.date))[0];
                    return (
                      <tr key={r.id}>
                        <td style={{ fontWeight: 800 }}>{r.roomNumber}</td>
                        <td>{r.name}</td>
                        <td>{latest?.date || '--'}</td>
                        <td style={{ color: 'var(--accent)', fontWeight: 700 }}>¥{latest?.totalAmount.toFixed(1) || '0.0'}</td>
                        <td>¥{latest?.amountPaid.toFixed(1) || '0.0'}</td>
                        <td><span className={`status-chip ${latest?.status === '已缴' ? 'paid' : 'pending'}`}>{latest?.status || '未入账'}</span></td>
                        <td>
                          <div className="action-group">
                            <button className="btn-icon" title="登记缴费" onClick={() => { setPayAmount((latest.totalAmount - latest.amountPaid).toFixed(2)); setModalState({ type: 'pay', record: latest }); }}><PayIcon /></button>
                            <button className="btn-icon" title="查看历史" onClick={() => { setHistoryRoomId(r.roomNumber); setActivePage('history'); }}><HistoryIcon /></button>
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {activePage === 'metering' && (
          <div className="view-fade-in">
            <header className="main-header">
              <div><h1>抄表录入</h1><p className="subtitle">更新当前最新度数并生成对应账单</p></div>
              <div className="search-box"><SearchIcon /><input type="text" placeholder="搜索房号..." value={searchQuery} onChange={e => setSearchQuery(e.target.value)} /></div>
            </header>
            <div className="management-grid">
              {uniqueTenants.filter(r => r.roomNumber.includes(searchQuery)).map(r => {
                const latest = records.filter(rec => rec.roomNumber === r.roomNumber).sort((a,b) => b.date.localeCompare(a.date))[0];
                return (
                  <div className="tenant-detail-card metering-style" key={r.id}>
                    <div className="tenant-card-header"><h3>房号: {r.roomNumber}</h3><span className="room-badge">{r.name}</span></div>
                    <div className="tenant-card-body">
                      <p><span>上次抄表日期</span> <b>{latest?.recordedAt || '无记录'}</b></p>
                      <p><span>当前水读数</span> <b style={{ color: 'var(--accent)' }}>{latest?.waterReading || 0} 吨</b></p>
                      <p><span>当前电读数</span> <b style={{ color: 'var(--warning)' }}>{latest?.electricityReading || 0} 度</b></p>
                    </div>
                    <div className="tenant-card-footer">
                      <button className="btn-primary" style={{ width: '100%' }} onClick={() => {
                        setMeterInput({ water: latest?.waterReading.toString() || '0', electricity: latest?.electricityReading.toString() || '0', date: new Date().toISOString().slice(0, 10) });
                        setModalState({ type: 'meter', record: latest });
                      }}><MeterIcon /> 录入新读数</button>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        )}

        {activePage === 'collection' && (
          <div className="view-fade-in">
            <header className="main-header"><div><h1>催缴任务</h1><p className="subtitle">快速生成 AI 催缴文案并登记收款</p></div></header>
            <div className="management-grid">
              {pendingCollection.map(r => (
                <div className="tenant-detail-card" key={r.id} style={{ borderLeft: `4px solid ${r.status === '部分缴纳' ? 'var(--accent)' : 'var(--warning)'}` }}>
                  <div className="tenant-card-header"><h3>{r.name} <small>{r.date}</small></h3><span className="room-badge">{r.roomNumber}</span></div>
                  <div className="tenant-card-body">
                    <p><span>实收额 / 应收额</span> <b>¥{r.amountPaid.toFixed(2)} / ¥{r.totalAmount.toFixed(2)}</b></p>
                    <p style={{ marginTop: 8, borderTop: '1px solid var(--border)', paddingTop: 8 }}><span>尚欠余额</span> <b style={{ color: 'var(--danger)', fontSize: '1.1rem' }}>¥{(r.totalAmount - r.amountPaid).toFixed(2)}</b></p>
                  </div>
                  <div className="tenant-card-footer">
                    <button className="btn-link" onClick={() => setModalState({ type: 'ai', record: r })}><SparkleIcon /> 智能提醒</button>
                    <button className="btn-primary" onClick={() => { setPayAmount((r.totalAmount - r.amountPaid).toFixed(2)); setModalState({ type: 'pay', record: r }); }}><PayIcon /> 登记缴费</button>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {activePage === 'management' && (
          <div className="view-fade-in">
            <header className="main-header">
              <div><h1>租户档案</h1><p className="subtitle">管理租户个人信息及长期合同状态</p></div>
              <button className="btn-primary" onClick={() => openEdit(null)}>登记新户</button>
            </header>
            <div className="management-grid">
              {uniqueTenants.map(r => (
                <div className="tenant-detail-card profile-card" key={r.id}>
                  <div className="tenant-card-header">
                    <h3>{r.name}</h3>
                    <span className="room-badge">{r.roomNumber}</span>
                  </div>
                  <div className="tenant-card-body">
                    <div className="info-row"><span className="label">联系方式</span> <span className="value">{r.phone || '--'}</span></div>
                    <div className="info-row"><span className="label">身份证</span> <span className="value">{r.idCard || '--'}</span></div>
                    <div className="info-row"><span className="label">押金</span> <span className="value">¥{r.deposit || 0}</span></div>
                    <div className="info-row"><span className="label">入住日期</span> <span className="value">{r.checkInDate || '--'}</span></div>
                  </div>
                  <div className="tenant-card-footer">
                    <button className="btn-link-action" onClick={() => openEdit(r)}>
                      <EditIcon /> <span>编辑</span>
                    </button>
                    <button className="btn-link-action highlight" onClick={() => { setHistoryRoomId(r.roomNumber); setActivePage('history'); }}>
                      <HistoryIcon /> <span>查看历史</span>
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {activePage === 'history' && (
          <div className="view-fade-in">
            <header className="main-header">
              <div><h1>房号 {historyRoomId} 审计记录</h1><p className="subtitle">展示历史读数变化及费用构成明细</p></div>
              <button className="btn-primary" onClick={() => setActivePage('management')}>返回档案</button>
            </header>
            <div className="table-wrapper">
              <table className="admin-table">
                <thead>
                  <tr>
                    <th>记录日期</th>
                    <th>水表变动</th>
                    <th>电表变动</th>
                    <th>费用明细</th>
                    <th>缴费状态</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {roomHistory.map(h => {
                    const prev = getPrevReading(h.roomNumber, h.date);
                    const waterUsage = prev ? (h.waterReading - prev.waterReading) : 0;
                    const elecUsage = prev ? (h.electricityReading - prev.electricityReading) : 0;
                    return (
                      <tr key={h.id}>
                        <td>
                          <div style={{ fontWeight: 700 }}>{h.date}</div>
                          <div style={{ fontSize: '0.65rem', color: 'var(--text-muted)' }}>{h.recordedAt}</div>
                        </td>
                        <td>
                          <div className="audit-detail">
                             <div className="reading-flow"><span>{prev?.waterReading || 0}</span> ➔ <b>{h.waterReading}</b></div>
                             <div className="usage-tag water">+{waterUsage.toFixed(1)} 吨</div>
                          </div>
                        </td>
                        <td>
                          <div className="audit-detail">
                             <div className="reading-flow"><span>{prev?.electricityReading || 0}</span> ➔ <b>{h.electricityReading}</b></div>
                             <div className="usage-tag electricity">+{elecUsage.toFixed(1)} 度</div>
                          </div>
                        </td>
                        <td>
                          <div style={{ fontSize: '0.75rem' }}>房租: ¥{h.rentAmount}</div>
                          <div style={{ fontWeight: 800 }}>总计: ¥{h.totalAmount.toFixed(1)}</div>
                        </td>
                        <td><span className={`status-chip ${h.status === '已缴' ? 'paid' : 'pending'}`}>{h.status}</span></td>
                        <td>
                          <div className="action-group">
                            <button className="btn-icon" title="修改记录" onClick={() => openEdit(h, 'history')}><EditIcon /></button>
                            <button className="btn-icon" title="删除记录" onClick={() => handleDeleteHistoryRecord(h)}><TrashIcon /></button>
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {activePage === 'income' && <IncomeStatsPanel />}
      </main>

      {/* 抄表录入模态框 */}
      {modalState.type === 'meter' && (
        <div className="modal-overlay" onClick={() => setModalState({ type: null, record: null })}>
          <div className="modal-content mini" onClick={e => e.stopPropagation()}>
            <div className="main-header"><h3>读数录入 - {modalState.record?.roomNumber}</h3></div>
            <form onSubmit={handleMeterSubmit}>
              <div className="form-group"><label>记录日期</label><input type="date" value={meterInput.date} onChange={e => setMeterInput({...meterInput, date: e.target.value})} /></div>
              <div className="form-group">
                <label>水表新读数 (吨)</label>
                <input required type="number" step="0.1" value={meterInput.water} onChange={e => setMeterInput({...meterInput, water: e.target.value})} />
              </div>
              <div className="form-group">
                <label>电表新读数 (度)</label>
                <input required type="number" step="0.1" value={meterInput.electricity} onChange={e => setMeterInput({...meterInput, electricity: e.target.value})} />
              </div>
              <div className="alert-box">系统将自动检索上一期读数并结算本次用量。</div>
              <button type="submit" className="btn-submit">提交并生成账单</button>
            </form>
          </div>
        </div>
      )}

      {/* 编辑/登记模态框 */}
      {modalState.type === 'edit' && (
        <div className="modal-overlay" onClick={() => setModalState({ type: null, record: null })}>
          <div className="modal-content large" onClick={e => e.stopPropagation()}>
            <form onSubmit={handleFormSubmit} className="form-grid-layout">
              <div className="form-section">
                <h3>租户基础信息</h3>
                <div className="form-group"><label>房号</label><input required value={formData.roomNumber} onChange={e => setFormData({...formData, roomNumber: e.target.value})} /></div>
                <div className="form-group"><label>租户姓名</label><input required value={formData.name} onChange={e => setFormData({...formData, name: e.target.value})} /></div>
                <div className="form-group"><label>联系电话</label><input value={formData.phone} onChange={e => setFormData({...formData, phone: e.target.value})} /></div>
                <div className="form-group"><label>身份证号</label><input value={formData.idCard} onChange={e => setFormData({...formData, idCard: e.target.value})} /></div>
                <div className="form-group"><label>入住日期</label><input type="date" value={formData.checkInDate} onChange={e => setFormData({...formData, checkInDate: e.target.value})} /></div>
              </div>
              <div className="form-section">
                <h3>账目设定</h3>
                <div className="form-group"><label>固定房租 (¥)</label><input type="number" value={formData.rentAmount} onChange={e => setFormData({...formData, rentAmount: e.target.value})} /></div>
                <div className="form-group">
                  <label>租金缴纳方式</label>
                  <select value={formData.rentCycle} onChange={e => setFormData({...formData, rentCycle: e.target.value as PaymentCycle})}>
                    <option value="月度">月度</option>
                    <option value="季度">季度</option>
                    <option value="年度">年度</option>
                  </select>
                </div>
                <div className="form-group"><label>当前水表读数</label><input type="number" step="0.1" value={formData.waterReading} onChange={e => setFormData({...formData, waterReading: e.target.value})} /></div>
                <div className="form-group"><label>当前电表读数</label><input type="number" step="0.1" value={formData.electricityReading} onChange={e => setFormData({...formData, electricityReading: e.target.value})} /></div>
                <div className="form-group"><label>合同押金 (¥)</label><input type="number" value={formData.deposit} onChange={e => setFormData({...formData, deposit: e.target.value})} /></div>
                <div style={{ marginTop: 24 }}>
                  <button type="submit" className="btn-submit">确认并保存档案</button>
                  <button type="button" className="btn-submit secondary-btn" onClick={() => setModalState({ type: null, record: null })}>取消</button>
                  {modalState.record && editMode === 'tenant' && (
                    <button type="button" className="btn-submit danger-btn" onClick={handleDeleteTenant} disabled={deletingTenant}>
                      <TrashIcon /> {deletingTenant ? '删除中...' : '删除租客'}
                    </button>
                  )}
                </div>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* 记账入账模态框 */}
      {modalState.type === 'pay' && (
        <div className="modal-overlay" onClick={() => setModalState({ type: null, record: null })}>
          <div className="modal-content mini" onClick={e => e.stopPropagation()}>
            <div className="main-header"><h3>登记实收金额</h3></div>
            <form onSubmit={handlePaySubmit}>
              <div className="form-group">
                <label>本次收款金额 (¥)</label>
                <input autoFocus type="number" step="0.01" value={payAmount} onChange={e => setPayAmount(e.target.value)} />
              </div>
              <button type="submit" className="btn-submit">确认入账</button>
            </form>
          </div>
        </div>
      )}

      {modalState.type === 'ai' && modalState.record && (
        <SmartReminderModal record={modalState.record} onClose={() => setModalState({ type: null, record: null })} />
      )}
    </div>
  );
}

ReactDOM.createRoot(document.getElementById('root')!).render(<App />);
