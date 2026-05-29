
/**
 * @license
 * SPDX-License-Identifier: Apache-2.0
*/

import { TenantRecord } from './types';

export const INITIAL_RECORDS: TenantRecord[] = [
    { id: '1', roomNumber: 'A-101', name: '张伟', phone: '13800138001', idCard: '440106199001015566', checkInDate: '2023-05-12', deposit: 2000, rentAmount: 2500, waterReading: 120.5, electricityReading: 450.2, waterBill: 82.5, electricityBill: 156.0, totalAmount: 2738.5, amountPaid: 2738.5, rentCycle: '月度', utilityCycle: '月度', status: '已缴', date: '2024-01', recordedAt: '2024-01-01' },
    { id: '2', roomNumber: 'A-102', name: '王芳', phone: '13911223344', idCard: '440106199205121122', checkInDate: '2023-06-01', deposit: 2500, rentAmount: 2800, waterReading: 80.0, electricityReading: 210.0, waterBill: 44.0, electricityBill: 112.8, totalAmount: 2956.8, amountPaid: 0, rentCycle: '月度', utilityCycle: '月度', status: '待缴', date: '2024-01', recordedAt: '2024-01-05' },
    { id: '3', roomNumber: 'B-201', name: '李强', phone: '13700137003', rentAmount: 3200, waterReading: 200.0, electricityReading: 600.0, waterBill: 121.0, electricityBill: 240.0, totalAmount: 3561.0, amountPaid: 1000, rentCycle: '季度', utilityCycle: '月度', status: '部分缴纳', date: '2024-01', recordedAt: '2024-01-02' }
];
