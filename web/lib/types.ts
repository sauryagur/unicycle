export type User={id:string;thapar_id:string;email:string;name:string;role:"student"|"admin";suspended:boolean;wallet_balance_paise:number;created_at:string};
export type BikeState="available"|"battery_disabled"|"ride_requested"|"unlocking"|"in_use"|"locking"|"lock_unconfirmed"|"ended"|"offline_ended"|"unlock_failed";
export type Bike={id:string;serial_number:string;mac_address:string;state:BikeState;battery_pct:number|null;last_seen_at:string|null;current_ride_id:string|null;disabled:boolean};
export type Ride={id:string;user_id:string;bike_id:string;started_at:string;ended_at:string|null;duration_seconds:number|null;amount_paise:number|null;state:"in_progress"|"ended"|"offline_ended";end_method?:"confirmed"|"offline_photo"|"admin_override";dispute_flag:boolean};
export type Transaction={id:string;amount_paise:number;type:"ride_charge"|"topup"|"refund"|"adjustment";description:string;balance_after_paise:number;created_at:string};
export type ReportType="damage"|"vandalism"|"malfunction"|"improper_parking";
