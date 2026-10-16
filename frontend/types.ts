
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
    rentAmount: number;        // Monthly rent
    rentDue?: number;          // Rent charged on this bill: rentAmount × cycle months in a cycle's first month, else 0
    waterReading: number;      // Current water meter reading
    electricityReading: number; // Current electricity meter reading
    waterBill: number;
    electricityBill: number;
    extraFee?: number;         // Other fees (garbage, repairs...)
    totalAmount: number;       // rentDue + waterBill + electricityBill + extraFee
    amountPaid: number;    
    rentCycle: PaymentCycle;    
    utilityCycle: PaymentCycle; 
    status: PaymentStatus;
    date: string; // Billing (meter) month, YYYY-MM
    dueMonth?: string; // Collection month: first collection month of the tenant's cycle on/after `date`
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

export interface RoomPaymentResult {
    roomNumber: string;
    applied: Record<string, number>; // bill id -> amount applied
    unapplied: number;               // left over after all unpaid bills were settled
}

export interface GenerateBillsResult {
    month: string;
    created: string[];  // room numbers that received a new bill
    skipped: number;    // rooms that already had a bill for the month
}

export interface Artifact {
    id: string;
    html: string;
    styleName: string;
    status: 'idle' | 'streaming' | 'complete';
}
