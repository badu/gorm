package gorm

type (
	expr struct {
		expr string
		args []interface{}
	}
)

// Expr generate raw SQL expression, for example:
//     DB.Model(&product).Update("price", gorm.Expr("price * ? + ?", 2, 100))
func Expr(expression string, args ...interface{}) *expr {
	return &expr{expr: expression, args: args}
}
//TODO : @Badu - move some pieces of code from Scope AddToVars, orderSQL, updatedAttrsWithValues
//TODO : @Badu - make expr string bytes buffer, allow args to be added, allow bytes buffer to be written into
/**
	var buf bytes.Buffer
	var prParams []interface{}
	if p.Id > 0 {
		buf.WriteString("%q:%d,")
		prParams = append(prParams, "id")
		prParams = append(prParams, p.Id)
	}
	buf.WriteString("%q:%q,%q:%q,%q:%t,%q:{%v}")
	prParams = append(prParams, "name")
	prParams = append(prParams, p.DisplayName)
	prParams = append(prParams, "states")
	prParams = append(prParams, p.USStates)
	prParams = append(prParams, "customerPays")
	prParams = append(prParams, p.AppliesToCustomer)
	prParams = append(prParams, "price")
	prParams = append(prParams, p.Price)
	return fmt.Sprintf(buf.String(), prParams...)
 */
//TODO : @Badu - use it to build strings with multiple fmt.Sprintf calls - making one call