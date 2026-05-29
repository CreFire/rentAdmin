
/**
 * @license
 * SPDX-License-Identifier: Apache-2.0
*/

export type PaymentStatus = '已缴' | '待缴' | '逾期' | '部分缴纳';
export type PaymentCycle = '月度' | '季度' | '半年' | '年度';

export interface TenantRecord {
    id: string;
    roomNumber: string;
    name: string;
    phone: string;
    idCard?: string;       
    checkInDate?: string;  
    deposit?: number;      
    rentAmount: number;
    waterReading: number;      // Current water meter reading
    electricityReading: number; // Current electricity meter reading
    waterBill: number;
    electricityBill: number;
    totalAmount: number;
    amountPaid: number;    
    rentCycle: PaymentCycle;    
    utilityCycle: PaymentCycle; 
    status: PaymentStatus;
    date: string; // Format YYYY-MM
    recordedAt?: string; // Specific date of reading
    monthlyIncome?: number; // Monthly income
    annualIncome?: number; // Annual income
    waterElecIncome?: number; // Water and electricity income
    monthlyWaterElecIncome?: number; // Monthly water and electricity income
    annualWaterElecIncome?: number; // Annual water and electricity income
}

export interface ExcelImportSummary {
    processedSheets: number;
    inserted: number;
    updated: number;
    skipped: number;
    errors: string[];
}

export interface Artifact {
    id: string;
    html: string;
    styleName: string;
    status: 'idle' | 'streaming' | 'complete';
}
