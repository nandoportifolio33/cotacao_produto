package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var db *gorm.DB
var productOptions []string
var productMap map[string]uint
var storeOptions []string
var storeMap map[string]uint
var productsList []Product
var storesList []Store
var quotesList []Quote
var prescriptionsList []Prescription

type User struct {
	gorm.Model
	Username string `gorm:"unique;not null"`
	Password string `gorm:"not null"`
	FullName string `gorm:"not null"`
	Email    string `gorm:"unique;not null"`
}

type Product struct {
	gorm.Model
	Name         string `gorm:"unique;not null"`
	StandardUnit string `gorm:"not null"`
}

type Store struct {
	gorm.Model
	Name     string `gorm:"unique;not null"`
	Endereco string `gorm:"unique;not null"`
	Telefone string `gorm:"unique"`
}

type Quote struct {
	gorm.Model
	ProductID        uint      `gorm:"not null"`
	StoreID          uint      `gorm:"not null"`
	Price            float64   `gorm:"not null"`
	PackagingSize    float64   `gorm:"not null"`
	PackagingUnit    string    `gorm:"not null"`
	ConversionFactor float64   `gorm:"not null;default:1.0"`
	Date             time.Time `gorm:"not null"`
	Product          Product   `gorm:"foreignKey:ProductID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	Store            Store     `gorm:"foreignKey:StoreID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}

type Prescription struct {
	gorm.Model
	ProductID        uint    `gorm:"not null"`
	RequiredQuantity float64 `gorm:"not null"`
	RequiredUnit     string  `gorm:"not null"`
	Product          Product `gorm:"foreignKey:ProductID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}

func Conectar() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Erro ao carregar .env:", err)
	}

	user := os.Getenv("DB_USER")
	pass := os.Getenv("DB_PASSWORD")
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	dbname := os.Getenv("DB_NAME")

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		host, user, pass, dbname, port,
	)

	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("Falha ao conectar ao banco de dados postgres: " + err.Error())
	}

	if err := db.AutoMigrate(&User{}, &Product{}, &Store{}, &Quote{}, &Prescription{}); err != nil {
		panic("Erro ao executar migração: " + err.Error())
	} else {
		fmt.Println("Conectado com sucesso. Migração concluída.")
	}

	var count int64
	db.Model(&User{}).Count(&count)
	if count == 0 {
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
		db.Create(&User{
			Username: "admin",
			Password: string(hashedPassword),
			FullName: "Administrador",
			Email:    "admin@example.com",
		})
		fmt.Println("Usuário padrão 'admin' criado com sucesso.")
	}
}

func main() {
	Conectar()
	productOptions, productMap = loadProductOptions()
	storeOptions, storeMap = loadStoreOptions()

	a := app.New()
	w := a.NewWindow("Sistema de Cotação de Produto Agricola")

	loginTab := loginScreen(w)
	w.SetContent(loginTab)
	w.Resize(fyne.NewSize(800, 600))
	w.ShowAndRun()
}

func loginScreen(w fyne.Window) fyne.CanvasObject {
	usernameEntry := widget.NewEntry()
	passwordEntry := widget.NewPasswordEntry()

	form := widget.NewForm(
		widget.NewFormItem("Usuário", usernameEntry),
		widget.NewFormItem("Senha", passwordEntry),
	)

	loginBtn := widget.NewButton("Login", func() {
		var user User
		if err := db.Where("username = ?", usernameEntry.Text).First(&user).Error; err != nil {
			dialog.ShowError(fmt.Errorf("Usuário não encontrado"), w)
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(passwordEntry.Text)); err != nil {
			dialog.ShowError(fmt.Errorf("Senha incorreta"), w)
			return
		}
		dialog.ShowInformation("Sucesso", "Login realizado!", w)
		tabs := container.NewAppTabs(
			container.NewTabItem("Produtos", productTab(w)),
			container.NewTabItem("Lojas", storeTab(w)),
			container.NewTabItem("Cotações", quoteTab(w)),
			container.NewTabItem("Receituários", prescriptionTab(w)),
			container.NewTabItem("Relatórios", reportTab(w)),
		)
		w.SetContent(tabs)
	})

	registerBtn := widget.NewButton("Cadastrar Novo Usuário", func() {
		w.SetContent(registerScreen(w))
	})

	return container.NewVBox(form, loginBtn, registerBtn)
}

func registerScreen(w fyne.Window) fyne.CanvasObject {
	usernameEntry := widget.NewEntry()
	fullNameEntry := widget.NewEntry()
	emailEntry := widget.NewEntry()
	passwordEntry := widget.NewPasswordEntry()
	confirmPasswordEntry := widget.NewPasswordEntry()

	form := widget.NewForm(
		widget.NewFormItem("Usuário", usernameEntry),
		widget.NewFormItem("Nome Completo", fullNameEntry),
		widget.NewFormItem("E-mail", emailEntry),
		widget.NewFormItem("Senha", passwordEntry),
		widget.NewFormItem("Confirmar Senha", confirmPasswordEntry),
	)

	registerBtn := widget.NewButton("Cadastrar", func() {
		if usernameEntry.Text == "" || fullNameEntry.Text == "" || emailEntry.Text == "" ||
			passwordEntry.Text == "" || confirmPasswordEntry.Text == "" {
			dialog.ShowError(fmt.Errorf("Todos os campos são obrigatórios"), w)
			return
		}
		if passwordEntry.Text != confirmPasswordEntry.Text {
			dialog.ShowError(fmt.Errorf("As senhas não coincidem"), w)
			return
		}
		if !strings.Contains(emailEntry.Text, "@") || !strings.Contains(emailEntry.Text, ".") {
			dialog.ShowError(fmt.Errorf("E-mail inválido"), w)
			return
		}
		var existingUser User
		if err := db.Where("username = ?", usernameEntry.Text).First(&existingUser).Error; err == nil {
			dialog.ShowError(fmt.Errorf("Nome de usuário já existe"), w)
			return
		}
		if err := db.Where("email = ?", emailEntry.Text).First(&existingUser).Error; err == nil {
			dialog.ShowError(fmt.Errorf("E-mail já registrado"), w)
			return
		}
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(passwordEntry.Text), bcrypt.DefaultCost)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Erro ao criptografar senha: %v", err), w)
			return
		}
		user := User{
			Username: usernameEntry.Text,
			FullName: fullNameEntry.Text,
			Email:    emailEntry.Text,
			Password: string(hashedPassword),
		}
		if err := db.Create(&user).Error; err != nil {
			dialog.ShowError(err, w)
			return
		}
		dialog.ShowInformation("Sucesso", "Usuário cadastrado com sucesso!", w)
		w.SetContent(loginScreen(w))
	})

	backBtn := widget.NewButton("Voltar ao Login", func() {
		w.SetContent(loginScreen(w))
	})

	return container.NewVBox(form, registerBtn, backBtn)
}

func loadProductOptions() ([]string, map[string]uint) {
	var products []Product
	db.Find(&products)
	productsList = products
	var options []string
	m := make(map[string]uint)
	for _, p := range products {
		opt := fmt.Sprintf("%d: %s (%s)", p.ID, p.Name, p.StandardUnit)
		options = append(options, opt)
		m[opt] = p.ID
	}
	return options, m
}

func loadStoreOptions() ([]string, map[string]uint) {
	var stores []Store
	db.Find(&stores)
	storesList = stores
	var options []string
	m := make(map[string]uint)
	for _, s := range stores {
		opt := fmt.Sprintf("%d: %s - %s - %s", s.ID, s.Name, s.Endereco, s.Telefone)
		options = append(options, opt)
		m[opt] = s.ID
	}
	return options, m
}

func updateComboBoxes(productSelect, storeSelect *widget.Select) {

	productOptions, productMap = loadProductOptions()
	storeOptions, storeMap = loadStoreOptions()

	productSelect.Options = productOptions
	productSelect.Selected = ""
	storeSelect.Options = storeOptions
	storeSelect.Selected = ""

	productSelect.Refresh()
	storeSelect.Refresh()
}

func productTab(w fyne.Window) fyne.CanvasObject {
	nameEntry := widget.NewEntry()
	unitEntry := widget.NewEntry()
	form := widget.NewForm(
		widget.NewFormItem("Nome do Produto", nameEntry),
		widget.NewFormItem("Unidade Padrão (KG/LT/etc)", unitEntry),
	)
	listData := binding.NewStringList()
	updateProductList(listData)

	addBtn := widget.NewButton("Adicionar Produto", func() {
		if nameEntry.Text == "" || unitEntry.Text == "" {
			dialog.ShowError(fmt.Errorf("Nome e unidade são obrigatórios"), w)
			return
		}
		product := Product{Name: nameEntry.Text, StandardUnit: unitEntry.Text}
		if err := db.Create(&product).Error; err != nil {
			dialog.ShowError(err, w)
			return
		}
		dialog.ShowInformation("Sucesso", "Produto adicionado!", w)
		nameEntry.SetText("")
		unitEntry.SetText("")
		updateProductList(listData)
	})

	var selectedProductIndex int = -1
	list := widget.NewListWithData(listData,
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(di binding.DataItem, co fyne.CanvasObject) {
			co.(*widget.Label).Bind(di.(binding.String))
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		selectedProductIndex = id
	}

	editBtn := widget.NewButton("Editar Produto Selecionado", func() {
		if selectedProductIndex < 0 || selectedProductIndex >= len(productsList) {
			dialog.ShowError(fmt.Errorf("Selecione um produto para editar"), w)
			return
		}
		product := productsList[selectedProductIndex]

		nameEdit := widget.NewEntry()
		nameEdit.SetText(product.Name)
		unitEdit := widget.NewEntry()
		unitEdit.SetText(product.StandardUnit)

		items := []*widget.FormItem{
			widget.NewFormItem("Nome do Produto", nameEdit),
			widget.NewFormItem("Unidade Padrão", unitEdit),
		}
		dlg := dialog.NewForm("Editar Produto", "Salvar", "Cancelar", items, func(ok bool) {
			if !ok {
				return
			}
			if nameEdit.Text == "" || unitEdit.Text == "" {
				dialog.ShowError(fmt.Errorf("Nome e unidade são obrigatórios"), w)
				return
			}
			product.Name = nameEdit.Text
			product.StandardUnit = unitEdit.Text
			if err := db.Save(&product).Error; err != nil {
				dialog.ShowError(err, w)
				return
			}
			dialog.ShowInformation("Sucesso", "Produto atualizado!", w)
			updateProductList(listData)
		}, w)
		dlg.Show()
	})

	deleteBtn := widget.NewButton("Deletar Produto Selecionado", func() {
		if selectedProductIndex < 0 || selectedProductIndex >= len(productsList) {
			dialog.ShowError(fmt.Errorf("Selecione um produto para deletar"), w)
			return
		}
		product := productsList[selectedProductIndex]
		dialog.ShowConfirm("Confirmação", "Tem certeza que deseja deletar este produto?", func(confirm bool) {
			if confirm {
				if err := db.Delete(&product).Error; err != nil {
					dialog.ShowError(err, w)
					return
				}
				dialog.ShowInformation("Sucesso", "Produto deletado!", w)
				updateProductList(listData)
			}
		}, w)
	})

	return container.NewVBox(form, addBtn, editBtn, deleteBtn, widget.NewLabel("Lista de Produtos:"), list)
}

func updateProductList(data binding.StringList) {
	var products []Product
	db.Find(&products)
	productsList = products
	var strs []string
	for _, p := range products {
		strs = append(strs, fmt.Sprintf("%d: %s (%s)", p.ID, p.Name, p.StandardUnit))
	}
	data.Set(strs)
}

func storeTab(w fyne.Window) fyne.CanvasObject {
	nameEntry := widget.NewEntry()
	enderecoEntry := widget.NewEntry()
	telefoneEntry := widget.NewEntry()
	form := widget.NewForm(
		widget.NewFormItem("Nome da Loja", nameEntry),
		widget.NewFormItem("Endereço", enderecoEntry),
		widget.NewFormItem("Telefone", telefoneEntry),
	)
	listData := binding.NewStringList()
	updateStoreList(listData)

	addBtn := widget.NewButton("Adicionar Loja", func() {
		if nameEntry.Text == "" || enderecoEntry.Text == "" {
			dialog.ShowError(fmt.Errorf("Nome e endereço da loja são obrigatórios"), w)
			return
		}
		store := Store{Name: nameEntry.Text, Endereco: enderecoEntry.Text, Telefone: telefoneEntry.Text}
		if err := db.Create(&store).Error; err != nil {
			dialog.ShowError(err, w)
			return
		}
		dialog.ShowInformation("Sucesso", "Loja adicionada!", w)
		nameEntry.SetText("")
		enderecoEntry.SetText("")
		telefoneEntry.SetText("")
		updateStoreList(listData)
	})

	var selectedStoreIndex int = -1
	list := widget.NewListWithData(listData,
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(di binding.DataItem, co fyne.CanvasObject) {
			co.(*widget.Label).Bind(di.(binding.String))
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		selectedStoreIndex = id
	}

	editBtn := widget.NewButton("Editar Loja Selecionada", func() {
		if selectedStoreIndex < 0 || selectedStoreIndex >= len(storesList) {
			dialog.ShowError(fmt.Errorf("Selecione uma loja para editar"), w)
			return
		}
		store := storesList[selectedStoreIndex]

		nameEdit := widget.NewEntry()
		nameEdit.SetText(store.Name)
		enderecoEdit := widget.NewEntry()
		enderecoEdit.SetText(store.Endereco)
		telefoneEdit := widget.NewEntry()
		telefoneEdit.SetText(store.Telefone)

		items := []*widget.FormItem{
			widget.NewFormItem("Nome da Loja", nameEdit),
			widget.NewFormItem("Endereço", enderecoEdit),
			widget.NewFormItem("Telefone", telefoneEdit),
		}
		dlg := dialog.NewForm("Editar Loja", "Salvar", "Cancelar", items, func(ok bool) {
			if !ok {
				return
			}
			if nameEdit.Text == "" || enderecoEdit.Text == "" {
				dialog.ShowError(fmt.Errorf("Nome e endereço são obrigatórios"), w)
				return
			}
			store.Name = nameEdit.Text
			store.Endereco = enderecoEdit.Text
			store.Telefone = telefoneEdit.Text
			if err := db.Save(&store).Error; err != nil {
				dialog.ShowError(err, w)
				return
			}
			dialog.ShowInformation("Sucesso", "Loja atualizada!", w)
			updateStoreList(listData)
		}, w)
		dlg.Show()
	})

	deleteBtn := widget.NewButton("Deletar Loja Selecionada", func() {
		if selectedStoreIndex < 0 || selectedStoreIndex >= len(storesList) {
			dialog.ShowError(fmt.Errorf("Selecione uma loja para deletar"), w)
			return
		}
		store := storesList[selectedStoreIndex]
		dialog.ShowConfirm("Confirmação", "Tem certeza que deseja deletar esta loja?", func(confirm bool) {
			if confirm {
				if err := db.Delete(&store).Error; err != nil {
					dialog.ShowError(err, w)
					return
				}
				dialog.ShowInformation("Sucesso", "Loja deletada!", w)
				updateStoreList(listData)
			}
		}, w)
	})

	return container.NewVBox(form, addBtn, editBtn, deleteBtn, widget.NewLabel("Lista de Lojas:"), list)
}

func updateStoreList(data binding.StringList) {
	var stores []Store
	db.Find(&stores)
	storesList = stores
	var strs []string
	for _, s := range stores {
		strs = append(strs, fmt.Sprintf("%d: %s - %s - %s", s.ID, s.Name, s.Endereco, s.Telefone))
	}
	data.Set(strs)
}

func quoteTab(w fyne.Window) fyne.CanvasObject {
	productSelect := widget.NewSelect(productOptions, func(s string) {})
	storeSelect := widget.NewSelect(storeOptions, func(s string) {})
	priceEntry := widget.NewEntry()
	packSizeEntry := widget.NewEntry()
	packUnitEntry := widget.NewEntry()
	convFactorEntry := widget.NewEntry()
	convFactorEntry.SetText("1.0")
	dateEntry := widget.NewEntry()

	form := widget.NewForm(
		widget.NewFormItem("Produto", productSelect),
		widget.NewFormItem("Loja", storeSelect),
		widget.NewFormItem("Preço por Embalagem (R$)", priceEntry),
		widget.NewFormItem("Tamanho da Embalagem", packSizeEntry),
		widget.NewFormItem("Unidade da Embalagem", packUnitEntry),
		widget.NewFormItem("Fator de Conversão Manual", convFactorEntry),
		widget.NewFormItem("Data (YYYY-MM-DD)", dateEntry),
	)
	listData := binding.NewStringList()
	updateQuoteList(listData)

	addBtn := widget.NewButton("Adicionar Cotação", func() {
		selectedProduct := productSelect.Selected
		if selectedProduct == "" {
			dialog.ShowError(fmt.Errorf("Selecione um produto"), w)
			return
		}
		productID, ok := productMap[selectedProduct]
		if !ok {
			dialog.ShowError(fmt.Errorf("Produto inválido"), w)
			return
		}
		selectedStore := storeSelect.Selected
		if selectedStore == "" {
			dialog.ShowError(fmt.Errorf("Selecione uma loja"), w)
			return
		}
		storeID, ok := storeMap[selectedStore]
		if !ok {
			dialog.ShowError(fmt.Errorf("Loja inválida"), w)
			return
		}
		price, err := strconv.ParseFloat(priceEntry.Text, 64)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Preço inválido"), w)
			return
		}
		packSize, err := strconv.ParseFloat(packSizeEntry.Text, 64)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Tamanho da embalagem inválido"), w)
			return
		}
		convFactor, err := strconv.ParseFloat(convFactorEntry.Text, 64)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Fator de conversão inválido"), w)
			return
		}
		if packUnitEntry.Text == "" {
			dialog.ShowError(fmt.Errorf("Unidade da embalagem é obrigatória"), w)
			return
		}
		dateStr := dateEntry.Text
		if dateStr == "" {
			dialog.ShowError(fmt.Errorf("Data é obrigatória"), w)
			return
		}
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Formato de data inválido (use YYYY-MM-DD)"), w)
			return
		}
		quote := Quote{
			ProductID:        productID,
			StoreID:          storeID,
			Price:            price,
			PackagingSize:    packSize,
			PackagingUnit:    packUnitEntry.Text,
			ConversionFactor: convFactor,
			Date:             t,
		}
		if err := db.Create(&quote).Error; err != nil {
			dialog.ShowError(err, w)
			return
		}
		dialog.ShowInformation("Sucesso", "Cotação adicionada!", w)
		productSelect.ClearSelected()
		storeSelect.ClearSelected()
		priceEntry.SetText("")
		packSizeEntry.SetText("")
		packUnitEntry.SetText("")
		convFactorEntry.SetText("1.0")
		dateEntry.SetText("")
		updateQuoteList(listData)
		updateComboBoxes(productSelect, storeSelect)
	})

	refreshBtn := widget.NewButton("Atualizar Listas de Produtos e Lojas", func() {
		updateComboBoxes(productSelect, storeSelect)
	})

	var selectedQuoteIndex int = -1
	list := widget.NewListWithData(listData,
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(di binding.DataItem, co fyne.CanvasObject) {
			co.(*widget.Label).Bind(di.(binding.String))
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		selectedQuoteIndex = id
	}

	editBtn := widget.NewButton("Editar Cotação Selecionada", func() {
		if selectedQuoteIndex < 0 || selectedQuoteIndex >= len(quotesList) {
			dialog.ShowError(fmt.Errorf("Selecione uma cotação para editar"), w)
			return
		}
		quote := quotesList[selectedQuoteIndex]

		updateComboBoxes(productSelect, storeSelect)

		productSelectEdit := widget.NewSelect(productOptions, func(s string) {})
		for opt, id := range productMap {
			if id == quote.ProductID {
				productSelectEdit.SetSelected(opt)
				break
			}
		}
		storeSelectEdit := widget.NewSelect(storeOptions, func(s string) {})
		for opt, id := range storeMap {
			if id == quote.StoreID {
				storeSelectEdit.SetSelected(opt)
				break
			}
		}
		priceEdit := widget.NewEntry()
		priceEdit.SetText(fmt.Sprintf("%.2f", quote.Price))
		packSizeEdit := widget.NewEntry()
		packSizeEdit.SetText(fmt.Sprintf("%.2f", quote.PackagingSize))
		packUnitEdit := widget.NewEntry()
		packUnitEdit.SetText(quote.PackagingUnit)
		convFactorEdit := widget.NewEntry()
		convFactorEdit.SetText(fmt.Sprintf("%.2f", quote.ConversionFactor))
		dateEdit := widget.NewEntry()
		dateEdit.SetText(quote.Date.Format("2006-01-02"))

		items := []*widget.FormItem{
			widget.NewFormItem("Produto", productSelectEdit),
			widget.NewFormItem("Loja", storeSelectEdit),
			widget.NewFormItem("Preço por Embalagem (R$)", priceEdit),
			widget.NewFormItem("Tamanho da Embalagem", packSizeEdit),
			widget.NewFormItem("Unidade da Embalagem", packUnitEdit),
			widget.NewFormItem("Fator de Conversão Manual", convFactorEdit),
			widget.NewFormItem("Data (YYYY-MM-DD)", dateEdit),
		}
		dlg := dialog.NewForm("Editar Cotação", "Salvar", "Cancelar", items, func(ok bool) {
			if !ok {
				return
			}
			selectedProduct := productSelectEdit.Selected
			if selectedProduct == "" {
				dialog.ShowError(fmt.Errorf("Selecione um produto"), w)
				return
			}
			productID, ok := productMap[selectedProduct]
			if !ok {
				dialog.ShowError(fmt.Errorf("Produto inválido"), w)
				return
			}
			selectedStore := storeSelectEdit.Selected
			if selectedStore == "" {
				dialog.ShowError(fmt.Errorf("Selecione uma loja"), w)
				return
			}
			storeID, ok := storeMap[selectedStore]
			if !ok {
				dialog.ShowError(fmt.Errorf("Loja inválida"), w)
				return
			}
			price, err := strconv.ParseFloat(priceEdit.Text, 64)
			if err != nil {
				dialog.ShowError(fmt.Errorf("Preço inválido"), w)
				return
			}
			packSize, err := strconv.ParseFloat(packSizeEdit.Text, 64)
			if err != nil {
				dialog.ShowError(fmt.Errorf("Tamanho da embalagem inválido"), w)
				return
			}
			convFactor, err := strconv.ParseFloat(convFactorEdit.Text, 64)
			if err != nil {
				dialog.ShowError(fmt.Errorf("Fator de conversão inválido"), w)
				return
			}
			if packUnitEdit.Text == "" {
				dialog.ShowError(fmt.Errorf("Unidade da embalagem é obrigatória"), w)
				return
			}
			dateStr := dateEdit.Text
			if dateStr == "" {
				dialog.ShowError(fmt.Errorf("Data é obrigatória"), w)
				return
			}
			t, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				dialog.ShowError(fmt.Errorf("Formato de data inválido (use YYYY-MM-DD)"), w)
				return
			}
			quote.ProductID = productID
			quote.StoreID = storeID
			quote.Price = price
			quote.PackagingSize = packSize
			quote.PackagingUnit = packUnitEdit.Text
			quote.ConversionFactor = convFactor
			quote.Date = t
			if err := db.Save(&quote).Error; err != nil {
				dialog.ShowError(err, w)
				return
			}
			dialog.ShowInformation("Sucesso", "Cotação atualizada!", w)
			updateQuoteList(listData)
			updateComboBoxes(productSelect, storeSelect)
		}, w)
		dlg.Show()
	})

	deleteBtn := widget.NewButton("Deletar Cotação Selecionada", func() {
		if selectedQuoteIndex < 0 || selectedQuoteIndex >= len(quotesList) {
			dialog.ShowError(fmt.Errorf("Selecione uma cotação para deletar"), w)
			return
		}
		quote := quotesList[selectedQuoteIndex]
		dialog.ShowConfirm("Confirmação", "Tem certeza que deseja deletar esta cotação?", func(confirm bool) {
			if confirm {
				if err := db.Delete(&quote).Error; err != nil {
					dialog.ShowError(err, w)
					return
				}
				dialog.ShowInformation("Sucesso", "Cotação deletada!", w)
				updateQuoteList(listData)
				updateComboBoxes(productSelect, storeSelect)
			}
		}, w)
	})

	return container.NewVBox(form, addBtn, refreshBtn, editBtn, deleteBtn, widget.NewLabel("Lista de Cotações:"), list)
}

func updateQuoteList(data binding.StringList) {
	var quotes []Quote
	db.Preload("Product").Preload("Store").Find(&quotes)
	quotesList = quotes
	var strs []string
	for _, q := range quotes {
		strs = append(strs, fmt.Sprintf("ID: %d, Prod: %s, Loja: %s, Preço: %.2f, Tam: %.2f %s, Conv: %.2f, Data: %s",
			q.ID, q.Product.Name, q.Store.Name, q.Price, q.PackagingSize, q.PackagingUnit, q.ConversionFactor, q.Date.Format("2006-01-02")))
	}
	data.Set(strs)
}

func prescriptionTab(w fyne.Window) fyne.CanvasObject {
	productSelect := widget.NewSelect(productOptions, func(s string) {})
	reqQtyEntry := widget.NewEntry()
	reqUnitEntry := widget.NewEntry()

	form := widget.NewForm(
		widget.NewFormItem("Produto", productSelect),
		widget.NewFormItem("Quantidade Requerida", reqQtyEntry),
		widget.NewFormItem("Unidade Requerida", reqUnitEntry),
	)
	listData := binding.NewStringList()
	updatePrescriptionList(listData)

	addBtn := widget.NewButton("Adicionar Receituário", func() {
		selectedProduct := productSelect.Selected
		if selectedProduct == "" {
			dialog.ShowError(fmt.Errorf("Selecione um produto"), w)
			return
		}
		productID, ok := productMap[selectedProduct]
		if !ok {
			dialog.ShowError(fmt.Errorf("Produto inválido"), w)
			return
		}
		reqQty, err := strconv.ParseFloat(reqQtyEntry.Text, 64)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Quantidade inválida"), w)
			return
		}
		if reqUnitEntry.Text == "" {
			dialog.ShowError(fmt.Errorf("Unidade requerida é obrigatória"), w)
			return
		}
		var product Product
		if err := db.First(&product, productID).Error; err != nil {
			dialog.ShowError(fmt.Errorf("Produto não encontrado"), w)
			return
		}
		if reqUnitEntry.Text != product.StandardUnit {
			dialog.ShowError(fmt.Errorf("Unidade requerida '%s' não compatível com unidade padrão '%s'", reqUnitEntry.Text, product.StandardUnit), w)
			return
		}
		pres := Prescription{
			ProductID:        productID,
			RequiredQuantity: reqQty,
			RequiredUnit:     reqUnitEntry.Text,
		}
		if err := db.Create(&pres).Error; err != nil {
			dialog.ShowError(err, w)
			return
		}
		dialog.ShowInformation("Sucesso", "Receituário adicionado!", w)
		productSelect.ClearSelected()
		reqQtyEntry.SetText("")
		reqUnitEntry.SetText("")
		updatePrescriptionList(listData)
		productOptions, productMap = loadProductOptions()
		productSelect.Options = productOptions
		productSelect.Refresh()
	})

	refreshBtn := widget.NewButton("Atualizar Lista de Produtos", func() {
		productOptions, productMap = loadProductOptions()
		productSelect.Options = productOptions
		productSelect.Refresh()
	})

	var selectedPrescriptionIndex int = -1
	list := widget.NewListWithData(listData,
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(di binding.DataItem, co fyne.CanvasObject) {
			co.(*widget.Label).Bind(di.(binding.String))
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		selectedPrescriptionIndex = id
	}

	editBtn := widget.NewButton("Editar Receituário Selecionado", func() {
		if selectedPrescriptionIndex < 0 || selectedPrescriptionIndex >= len(prescriptionsList) {
			dialog.ShowError(fmt.Errorf("Selecione um receituário para editar"), w)
			return
		}
		pres := prescriptionsList[selectedPrescriptionIndex]

		productOptions, productMap = loadProductOptions()

		productSelectEdit := widget.NewSelect(productOptions, func(s string) {})
		for opt, id := range productMap {
			if id == pres.ProductID {
				productSelectEdit.SetSelected(opt)
				break
			}
		}
		reqQtyEdit := widget.NewEntry()
		reqQtyEdit.SetText(fmt.Sprintf("%.2f", pres.RequiredQuantity))
		reqUnitEdit := widget.NewEntry()
		reqUnitEdit.SetText(pres.RequiredUnit)

		items := []*widget.FormItem{
			widget.NewFormItem("Produto", productSelectEdit),
			widget.NewFormItem("Quantidade Requerida", reqQtyEdit),
			widget.NewFormItem("Unidade Requerida", reqUnitEdit),
		}
		dlg := dialog.NewForm("Editar Receituário", "Salvar", "Cancelar", items, func(ok bool) {
			if !ok {
				return
			}
			selectedProduct := productSelectEdit.Selected
			if selectedProduct == "" {
				dialog.ShowError(fmt.Errorf("Selecione um produto"), w)
				return
			}
			productID, ok := productMap[selectedProduct]
			if !ok {
				dialog.ShowError(fmt.Errorf("Produto inválido"), w)
				return
			}
			reqQty, err := strconv.ParseFloat(reqQtyEdit.Text, 64)
			if err != nil {
				dialog.ShowError(fmt.Errorf("Quantidade inválida"), w)
				return
			}
			if reqUnitEdit.Text == "" {
				dialog.ShowError(fmt.Errorf("Unidade requerida é obrigatória"), w)
				return
			}
			var product Product
			if err := db.First(&product, productID).Error; err != nil {
				dialog.ShowError(fmt.Errorf("Produto não encontrado"), w)
				return
			}
			if reqUnitEdit.Text != product.StandardUnit {
				dialog.ShowError(fmt.Errorf("Unidade requerida '%s' não compatível com unidade padrão '%s'", reqUnitEdit.Text, product.StandardUnit), w)
				return
			}
			pres.ProductID = productID
			pres.RequiredQuantity = reqQty
			pres.RequiredUnit = reqUnitEdit.Text
			if err := db.Save(&pres).Error; err != nil {
				dialog.ShowError(err, w)
				return
			}
			dialog.ShowInformation("Sucesso", "Receituário atualizado!", w)
			updatePrescriptionList(listData)
			productOptions, productMap = loadProductOptions()
			productSelect.Options = productOptions
			productSelect.Refresh()
		}, w)
		dlg.Show()
	})

	deleteBtn := widget.NewButton("Deletar Receituário Selecionado", func() {
		if selectedPrescriptionIndex < 0 || selectedPrescriptionIndex >= len(prescriptionsList) {
			dialog.ShowError(fmt.Errorf("Selecione um receituário para deletar"), w)
			return
		}
		pres := prescriptionsList[selectedPrescriptionIndex]
		dialog.ShowConfirm("Confirmação", "Tem certeza que deseja deletar este receituário?", func(confirm bool) {
			if confirm {
				if err := db.Delete(&pres).Error; err != nil {
					dialog.ShowError(err, w)
					return
				}
				dialog.ShowInformation("Sucesso", "Receituário deletado!", w)
				updatePrescriptionList(listData)
				productOptions, productMap = loadProductOptions()
				productSelect.Options = productOptions
				productSelect.Refresh()
			}
		}, w)
	})

	return container.NewVBox(form, addBtn, refreshBtn, editBtn, deleteBtn, widget.NewLabel("Lista de Receituários:"), list)
}

func updatePrescriptionList(data binding.StringList) {
	var pres []Prescription
	db.Preload("Product").Find(&pres)
	prescriptionsList = pres
	var strs []string
	for _, p := range pres {
		strs = append(strs, fmt.Sprintf("%d: %s - %.2f %s", p.ID, p.Product.Name, p.RequiredQuantity, p.RequiredUnit))
	}
	data.Set(strs)
}

func reportTab(w fyne.Window) fyne.CanvasObject {
	dateEntry := widget.NewEntry()
	dateEntry.SetPlaceHolder("YYYY-MM-DD")
	form := widget.NewForm(
		widget.NewFormItem("Data", dateEntry),
	)
	reportLabel := widget.NewLabel("")
	fullReportLabel := widget.NewLabel("")

	genBtn := widget.NewButton("Gerar Relatório por Data", func() {
		dateStr := dateEntry.Text
		if dateStr == "" {
			dialog.ShowError(fmt.Errorf("Data é obrigatória"), w)
			return
		}
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Formato de data inválido (use YYYY-MM-DD)"), w)
			return
		}
		report := generateReportByDate(t)
		reportLabel.SetText(report)
	})

	showAllBtn := widget.NewButton("Mostrar Vencedores e Perdedores", func() {
		dateStr := dateEntry.Text
		if dateStr == "" {
			dialog.ShowError(fmt.Errorf("Data é obrigatória"), w)
			return
		}
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Formato de data inválido (use YYYY-MM-DD)"), w)
			return
		}
		fullReport := generateFullReportByDate(t)
		fullReportLabel.SetText(fullReport)
	})

	return container.NewVBox(form, genBtn, reportLabel, showAllBtn, fullReportLabel)
}

func generateReportByDate(date time.Time) string {
	var prescriptions []Prescription
	db.Preload("Product").Find(&prescriptions)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Relatório de Cotações Vencedoras para %s:\n\n", date.Format("2006-01-02")))

	for _, pres := range prescriptions {
		if pres.Product.ID == 0 {
			sb.WriteString(fmt.Sprintf("Produto com ID %d não encontrado.\n", pres.ProductID))
			continue
		}

		if pres.RequiredUnit != pres.Product.StandardUnit {
			sb.WriteString(fmt.Sprintf("Unidade requerida '%s' não combina com padrão '%s' para '%s'.\n", pres.RequiredUnit, pres.Product.StandardUnit, pres.Product.Name))
			continue
		}

		var quotes []Quote
		db.Preload("Store").Where("product_id = ? AND date = ?", pres.ProductID, date).Find(&quotes)

		if len(quotes) == 0 {
			sb.WriteString(fmt.Sprintf("Nenhuma cotação para '%s' na data %s.\n", pres.Product.Name, date.Format("2006-01-02")))
			continue
		}

		minCost := float64(999999999)
		var bestQuote Quote
		var bestStore Store

		for _, quote := range quotes {
			pricePerStandard := quote.Price / (quote.PackagingSize * quote.ConversionFactor)
			totalCost := pricePerStandard * pres.RequiredQuantity

			if totalCost < minCost {
				minCost = totalCost
				bestQuote = quote
				bestStore = quote.Store
			}
		}

		if bestQuote.ID != 0 {
			sb.WriteString(fmt.Sprintf("Para '%s' (%.2f %s):\n", pres.Product.Name, pres.RequiredQuantity, pres.RequiredUnit))
			sb.WriteString(fmt.Sprintf("  Vencedor: Loja '%s' (%s) - Custo Total: R$ %.2f\n", bestStore.Name, bestStore.Endereco, minCost))
			sb.WriteString(fmt.Sprintf("  Detalhes: Preço R$ %.2f por %.2f %s (Conv: %.2f) em %s\n\n", bestQuote.Price, bestQuote.PackagingSize, bestQuote.PackagingUnit, bestQuote.ConversionFactor, bestQuote.Date.Format("2006-01-02")))
		}
	}

	return sb.String()
}

func generateFullReportByDate(date time.Time) string {
	var prescriptions []Prescription
	db.Preload("Product").Find(&prescriptions)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Relatório Completo de Cotações (Vencedores e Perdedores) para %s:\n\n", date.Format("2006-01-02")))

	for _, pres := range prescriptions {
		if pres.Product.ID == 0 {
			sb.WriteString(fmt.Sprintf("Produto com ID %d não encontrado.\n", pres.ProductID))
			continue
		}

		if pres.RequiredUnit != pres.Product.StandardUnit {
			sb.WriteString(fmt.Sprintf("Unidade requerida '%s' não combina com padrão '%s' para '%s'.\n", pres.RequiredUnit, pres.Product.StandardUnit, pres.Product.Name))
			continue
		}

		var quotes []Quote
		db.Preload("Store").Where("product_id = ? AND date = ?", pres.ProductID, date).Find(&quotes)

		if len(quotes) == 0 {
			sb.WriteString(fmt.Sprintf("Nenhuma cotação para '%s' na data %s.\n", pres.Product.Name, date.Format("2006-01-02")))
			continue
		}

		type quoteCost struct {
			quote Quote
			cost  float64
		}
		var costs []quoteCost
		for _, quote := range quotes {
			pricePerStandard := quote.Price / (quote.PackagingSize * quote.ConversionFactor)
			totalCost := pricePerStandard * pres.RequiredQuantity
			costs = append(costs, quoteCost{quote: quote, cost: totalCost})
		}

		for i := range costs {
			for j := i + 1; j < len(costs); j++ {
				if costs[i].cost > costs[j].cost {
					costs[i], costs[j] = costs[j], costs[i]
				}
			}
		}

		sb.WriteString(fmt.Sprintf("Para '%s' (%.2f %s):\n", pres.Product.Name, pres.RequiredQuantity, pres.RequiredUnit))
		for idx, qc := range costs {
			status := "Perdedor"
			if idx == 0 {
				status = "Vencedor"
			}
			sb.WriteString(fmt.Sprintf("  %s: Loja '%s' (%s) - Custo Total: R$ %.2f\n", status, qc.quote.Store.Name, qc.quote.Store.Endereco, qc.cost))
			sb.WriteString(fmt.Sprintf("    Detalhes: Preço R$ %.2f por %.2f %s (Conv: %.2f) em %s\n", qc.quote.Price, qc.quote.PackagingSize, qc.quote.PackagingUnit, qc.quote.ConversionFactor, qc.quote.Date.Format("2006-01-02")))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
