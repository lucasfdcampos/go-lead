import csv

# Ler CNPJ
cnpj_data = {}
with open('find-cnpj/resultados_cnpj.csv', 'r', encoding='utf-8') as f:
    reader = csv.DictReader(f)
    for row in reader:
        nome = row['Nome'].strip().lower()
        cnpj_data[nome] = {
            'cnpj': row['CNPJ_Formatado'],
            'razao_social': row['Razao_Social'],
            'nome_fantasia': row['Nome_Fantasia'],
            'telefones': row['Telefones'],
            'socios': row['Socios']
        }

# Ler Instagram
instagram_data = {}
with open('find-instagram/resultados_instagram.csv', 'r', encoding='utf-8') as f:
    reader = csv.DictReader(f)
    for row in reader:
        nome = row['Nome'].strip().lower()
        instagram_data[nome] = {
            'handle': row['Handle'],
            'url': row['URL'],
            'followers': row['Followers']
        }

# Consolidar
with open('resultados_consolidados.csv', 'w', encoding='utf-8', newline='') as f:
    writer = csv.writer(f)
    writer.writerow([
        'Nome', 'CNPJ', 'Razao_Social', 'Nome_Fantasia', 
        'Telefones', 'Socios', 'Instagram_Handle', 
        'Instagram_URL', 'Seguidores'
    ])
    
    for nome in cnpj_data.keys():
        cnpj = cnpj_data.get(nome, {})
        insta = instagram_data.get(nome, {})
        
        writer.writerow([
            nome.title(),
            cnpj.get('cnpj', ''),
            cnpj.get('razao_social', ''),
            cnpj.get('nome_fantasia', ''),
            cnpj.get('telefones', ''),
            cnpj.get('socios', ''),
            insta.get('handle', ''),
            insta.get('url', ''),
            insta.get('followers', '')
        ])

print("✅ CSV consolidado criado!")
