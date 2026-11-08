SELECT
    cd.company_name, cd.company_number, cd.reg_address_care_of, cd.reg_address_po_box,
    cd.reg_address_address_line_1, cd.reg_address_address_line_2, cd.reg_address_post_town,
    cd.reg_address_county, cd.reg_address_country, cd.reg_address_post_code,
    cd.company_category, cd.company_status, cd.country_of_origin, cd.dissolution_date,
    cd.incorporation_date, cd.accounts_account_ref_day, cd.accounts_account_ref_month,
    cd.accounts_next_due_date, cd.accounts_last_made_up_date, cd.accounts_account_category,
    cd.returns_next_due_date, cd.returns_last_made_up_date, cd.mortgages_num_charges,
    cd.mortgages_num_outstanding, cd.mortgages_num_part_satisfied, cd.mortgages_num_satisfied,
    cd.sic_code_1, cd.sic_code_2, cd.sic_code_3, cd.sic_code_4,
    cd.limited_partnerships_num_gen_partners, cd.limited_partnerships_num_lim_partners,
    cd.conf_stmt_next_due_date, cd.conf_stmt_last_made_up_date,
    cp.easting, cp.northing
FROM code_point cp
INNER JOIN company_data cd ON cp.post_code = cd.reg_address_post_code
WHERE cp.easting BETWEEN ? AND ?
AND cp.northing BETWEEN ? AND ?
