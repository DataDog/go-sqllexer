SELECT 
  e1.name AS employee_name,
  e1.salary,
  e2.name AS manager_name,
  AVG(e2.salary) OVER (PARTITION BY e1.manager_id) AS avg_manager_salary,
  RANK() OVER (ORDER BY e1.salary DESC) AS salary_rank
FROM 
  employees e1
LEFT JOIN employees e2 ON e1.manager_id = e2.id
WHERE 
  e1.department_id IN (SELECT id FROM departments WHERE name LIKE 'IT%')
AND 
  e1.hire_date > '2020-01-01'
ORDER BY 
  salary_rank, avg_manager_salary DESC;
